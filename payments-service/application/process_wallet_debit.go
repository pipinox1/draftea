package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessWalletDebitCommand represents the command to process wallet debit response
type ProcessWalletDebitCommand struct {
	PaymentID     models.ID    `json:"payment_id"`
	WalletID      string       `json:"wallet_id"`
	TransactionID string       `json:"transaction_id"`
	Amount        models.Money `json:"amount"`
	Status        string       `json:"status"` // "completed" or "failed"
	ErrorCode     string       `json:"error_code,omitempty"`
	ErrorMessage  string       `json:"error_message,omitempty"`
}

// ProcessWalletDebit use case handles wallet debit responses and converts them to payment operations
type ProcessWalletDebit struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessWalletDebit creates a new ProcessWalletDebit use case
func NewProcessWalletDebit(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessWalletDebit {
	return &ProcessWalletDebit{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes wallet debit response and creates corresponding payment operation
func (uc *ProcessWalletDebit) Execute(ctx context.Context, cmd *ProcessWalletDebitCommand) error {
	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		return errors.Wrap(err, "invalid command")
	}

	// Find payment
	payment, err := uc.paymentRepository.FindByID(ctx, cmd.PaymentID)
	if err != nil {
		return errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return errors.New("payment not found")
	}

	// Verify this is a wallet payment
	if payment.PaymentMethod.PaymentMethodType != "wallet" {
		return errors.New("payment is not a wallet payment")
	}

	// Create payment operation based on wallet response
	var operation *domain.PaymentOperation

	if cmd.Status == "completed" {
		// Create successful debit operation
		operation = domain.NewPaymentOperation(
			payment.ID,
			domain.PaymentOperationTypeDebit,
			cmd.Amount,
			"wallet",
		)

		// Complete the operation with wallet transaction details
		operation.Complete(cmd.TransactionID, cmd.WalletID)

	} else {
		// Create failed debit operation
		operation = domain.NewPaymentOperation(
			payment.ID,
			domain.PaymentOperationTypeDebit,
			cmd.Amount,
			"wallet",
		)

		// Fail the operation with error details
		operation.Fail(cmd.ErrorCode, cmd.ErrorMessage)
	}

	// Publish payment operation events
	if err := uc.eventPublisher.Publish(ctx, operation.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish payment operation events")
	}

	// Clear operation events
	operation.ClearEvents()

	return nil
}

// validateCommand validates the process wallet debit command
func (uc *ProcessWalletDebit) validateCommand(cmd *ProcessWalletDebitCommand) error {
	if cmd.PaymentID.String() == "" {
		return errors.New("payment ID is required")
	}

	if cmd.WalletID == "" {
		return errors.New("wallet ID is required")
	}

	if cmd.Amount.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if cmd.Status == "" {
		return errors.New("status is required")
	}

	if cmd.Status != "completed" && cmd.Status != "failed" {
		return errors.New("status must be either 'completed' or 'failed'")
	}

	if cmd.Status == "completed" && cmd.TransactionID == "" {
		return errors.New("transaction ID is required for completed operations")
	}

	if cmd.Status == "failed" && cmd.ErrorCode == "" {
		return errors.New("error code is required for failed operations")
	}

	return nil
}