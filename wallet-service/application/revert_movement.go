package application

import (
	"context"
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/draftea/payment-system/wallet-service/domain"
	"github.com/draftea/payment-system/shared/telemetry"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RevertMovementCommand represents the command to revert a wallet movement
type RevertMovementCommand struct {
	MovementID  string `json:"movement_id"`
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by"`
}

// RevertMovementResponse represents the response after reverting a movement
type RevertMovementResponse struct {
	ReversalTransactionID string       `json:"reversal_transaction_id"`
	OriginalTransactionID string       `json:"original_transaction_id"`
	WalletID              string       `json:"wallet_id"`
	Amount                models.Money `json:"amount"`
	BalanceAfter          models.Money `json:"balance_after"`
}

// RevertMovement use case handles reverting wallet movements by creating opposite movements
type RevertMovement struct {
	walletRepository      domain.WalletRepository
	transactionRepository domain.TransactionRepository
	eventPublisher        events.Publisher
}

// NewRevertMovement creates a new RevertMovement use case
func NewRevertMovement(
	walletRepository domain.WalletRepository,
	transactionRepository domain.TransactionRepository,
	eventPublisher events.Publisher,
) *RevertMovement {
	return &RevertMovement{
		walletRepository:      walletRepository,
		transactionRepository: transactionRepository,
		eventPublisher:        eventPublisher,
	}
}

// Execute reverts a wallet movement by creating the opposite movement
func (uc *RevertMovement) Execute(ctx context.Context, cmd *RevertMovementCommand) (*RevertMovementResponse, error) {
	// Start tracing span
	start := time.Now()
	ctx, span := telemetry.StartSpan(ctx, "revert_movement",
		trace.WithAttributes(
			attribute.String("movement_id", cmd.MovementID),
			attribute.String("reason", cmd.Reason),
		),
	)
	defer span.End()

	var status string = "error"
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCounter(ctx, "wallet_operations_total", "Total wallet operations", 1,
			attribute.String("operation", "revert_movement"),
			attribute.String("status", status),
		)
		telemetry.RecordHistogram(ctx, "wallet_operation_duration_seconds", "Wallet operation duration", duration.Seconds(),
			attribute.String("operation", "revert_movement"),
			attribute.String("status", status),
		)
	}()

	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "invalid command")
	}

	// Parse movement (transaction) ID
	movementID, err := models.NewID(cmd.MovementID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid movement ID")
	}

	// Find the original transaction by looking through wallets
	// In a real system, you'd have a proper transaction lookup method
	originalTransaction, wallet, err := uc.findTransactionAndWallet(ctx, movementID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find original transaction")
	}

	if originalTransaction == nil {
		return nil, errors.New("original transaction not found")
	}

	// Validate that movement can be reverted
	if err := uc.validateRevertEligibility(originalTransaction, wallet); err != nil {
		return nil, errors.Wrap(err, "movement not eligible for revert")
	}

	// Create the opposite movement
	var reversalTransaction *domain.Transaction
	reference := "Revert: " + originalTransaction.Reference + " - " + cmd.Reason

	switch originalTransaction.Type {
	case domain.TransactionTypeCredit:
		// Original was credit (income), so we debit (create expense)
		if originalTransaction.PaymentID == nil {
			// For credits without payment ID, we need to create a temporary one for the debit
			tempPaymentID := models.GenerateUUID()
			reversalTransaction, err = wallet.Debit(originalTransaction.Amount, tempPaymentID, reference)
		} else {
			reversalTransaction, err = wallet.Debit(originalTransaction.Amount, *originalTransaction.PaymentID, reference)
		}

	case domain.TransactionTypeDebit:
		// Original was debit (expense), so we credit (create income)
		reversalTransaction, err = wallet.Credit(originalTransaction.Amount, reference, originalTransaction.PaymentID)

	case domain.TransactionTypeRefund:
		// Original was refund (credit), so we debit to reverse it
		if originalTransaction.PaymentID == nil {
			tempPaymentID := models.GenerateUUID()
			reversalTransaction, err = wallet.Debit(originalTransaction.Amount, tempPaymentID, reference)
		} else {
			reversalTransaction, err = wallet.Debit(originalTransaction.Amount, *originalTransaction.PaymentID, reference)
		}

	default:
		return nil, errors.Errorf("unsupported transaction type for revert: %s", originalTransaction.Type)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to create reversal transaction")
	}

	// Update the reversal transaction to mark it as a reversal
	// Note: We'd need to extend the Transaction model to support this properly
	// For now, we'll use the reference field to indicate it's a reversal

	// Save wallet with updated balance
	if err := uc.walletRepository.Save(ctx, wallet); err != nil {
		return nil, errors.Wrap(err, "failed to save wallet")
	}

	// Save reversal transaction
	if err := uc.transactionRepository.Save(ctx, reversalTransaction); err != nil {
		return nil, errors.Wrap(err, "failed to save reversal transaction")
	}

	// Publish domain events from wallet
	if err := uc.eventPublisher.Publish(ctx, wallet.Events()...); err != nil {
		return nil, errors.Wrap(err, "failed to publish wallet events")
	}

	// Clear wallet events
	wallet.ClearEvents()

	// Publish movement reverted event
	revertedEvent := events.NewEvent(wallet.ID, events.WalletMovementRevertedEvent, WalletMovementRevertedData{
		WalletID:              wallet.ID,
		UserID:                wallet.UserID,
		OriginalTransactionID: originalTransaction.ID,
		ReversalTransactionID: reversalTransaction.ID,
		OriginalType:          string(originalTransaction.Type),
		Amount:                originalTransaction.Amount,
		BalanceBefore:         reversalTransaction.BalanceBefore,
		BalanceAfter:          reversalTransaction.BalanceAfter,
		Reason:                cmd.Reason,
		RequestedBy:           cmd.RequestedBy,
		PaymentID:             originalTransaction.PaymentID,
	})

	if err := uc.eventPublisher.Publish(ctx, revertedEvent); err != nil {
		return nil, errors.Wrap(err, "failed to publish movement reverted event")
	}

	status = "success"
	return &RevertMovementResponse{
		ReversalTransactionID: reversalTransaction.ID.String(),
		OriginalTransactionID: originalTransaction.ID.String(),
		WalletID:              wallet.ID.String(),
		Amount:                reversalTransaction.Amount,
		BalanceAfter:          wallet.Balance,
	}, nil
}

// findTransactionAndWallet finds a transaction and its wallet
func (uc *RevertMovement) findTransactionAndWallet(ctx context.Context, transactionID models.ID) (*domain.Transaction, *domain.Wallet, error) {
	// Find the transaction by ID using the repository
	transaction, err := uc.transactionRepository.FindByID(ctx, transactionID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to find transaction")
	}

	if transaction == nil {
		return nil, nil, nil
	}

	// Find the wallet that owns this transaction
	wallet, err := uc.walletRepository.FindByID(ctx, transaction.WalletID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to find wallet for transaction")
	}

	return transaction, wallet, nil
}

// validateRevertEligibility checks if a movement can be reverted
func (uc *RevertMovement) validateRevertEligibility(transaction *domain.Transaction, wallet *domain.Wallet) error {
	// Check if wallet is in a state that allows transactions
	if wallet.Status == domain.WalletStatusClosed {
		return errors.New("cannot revert movements on closed wallet")
	}

	// For debit reversals (which become credits), any wallet state is OK
	// For credit reversals (which become debits), check if wallet has sufficient funds
	if transaction.Type == domain.TransactionTypeCredit || transaction.Type == domain.TransactionTypeRefund {
		// This will become a debit, so check sufficient funds
		if !wallet.CanDebit(transaction.Amount) {
			return errors.New("insufficient funds to revert this movement")
		}
	}

	// TODO: Add additional business rules:
	// - Time-based revert policies
	// - Check if movement was already reverted
	// - Check for dependent transactions
	// - Business-specific revert rules

	return nil
}

// validateCommand validates the revert movement command
func (uc *RevertMovement) validateCommand(cmd *RevertMovementCommand) error {
	if cmd.MovementID == "" {
		return errors.New("movement ID is required")
	}

	if cmd.Reason == "" {
		return errors.New("reason is required")
	}

	if cmd.RequestedBy == "" {
		return errors.New("requested by is required")
	}

	return nil
}

// WalletMovementRevertedData represents data for wallet movement reverted event
type WalletMovementRevertedData struct {
	WalletID              models.ID    `json:"wallet_id"`
	UserID                models.ID    `json:"user_id"`
	OriginalTransactionID models.ID    `json:"original_transaction_id"`
	ReversalTransactionID models.ID    `json:"reversal_transaction_id"`
	OriginalType          string       `json:"original_type"`
	Amount                models.Money `json:"amount"`
	BalanceBefore         models.Money `json:"balance_before"`
	BalanceAfter          models.Money `json:"balance_after"`
	Reason                string       `json:"reason"`
	RequestedBy           string       `json:"requested_by"`
	PaymentID             *models.ID   `json:"payment_id,omitempty"`
}