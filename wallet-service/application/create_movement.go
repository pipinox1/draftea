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

// CreateMovementCommand represents the command to create a wallet movement
type CreateMovementCommand struct {
	WalletID    string    `json:"wallet_id"`
	Type        string    `json:"type"` // "income" or "expense"
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	Reference   string    `json:"reference"`
	PaymentID   string    `json:"payment_id,omitempty"`
	Description string    `json:"description,omitempty"`
}

// CreateMovementResponse represents the response after creating a movement
type CreateMovementResponse struct {
	TransactionID string       `json:"transaction_id"`
	WalletID      string       `json:"wallet_id"`
	Type          string       `json:"type"`
	Amount        models.Money `json:"amount"`
	BalanceAfter  models.Money `json:"balance_after"`
}

// CreateMovement use case handles creating wallet movements (income and expense)
type CreateMovement struct {
	walletRepository      domain.WalletRepository
	transactionRepository domain.TransactionRepository
	eventPublisher        events.Publisher
}

// NewCreateMovement creates a new CreateMovement use case
func NewCreateMovement(
	walletRepository domain.WalletRepository,
	transactionRepository domain.TransactionRepository,
	eventPublisher events.Publisher,
) *CreateMovement {
	return &CreateMovement{
		walletRepository:      walletRepository,
		transactionRepository: transactionRepository,
		eventPublisher:        eventPublisher,
	}
}

// Execute creates a wallet movement (income or expense)
func (uc *CreateMovement) Execute(ctx context.Context, cmd *CreateMovementCommand) (*CreateMovementResponse, error) {
	// Start tracing span
	start := time.Now()
	ctx, span := telemetry.StartSpan(ctx, "create_movement",
		trace.WithAttributes(
			attribute.String("wallet_id", cmd.WalletID),
			attribute.String("movement_type", cmd.Type),
			attribute.Int64("amount", cmd.Amount),
			attribute.String("currency", cmd.Currency),
		),
	)
	defer span.End()

	var status string = "error" // Default to error, set to success at the end
	defer func() {
		duration := time.Since(start)

		// Record operation counter
		telemetry.RecordCounter(ctx, "wallet_operations_total", "Total wallet operations", 1,
			attribute.String("operation", "create_movement"),
			attribute.String("status", status),
		)

		// Record operation duration
		telemetry.RecordHistogram(ctx, "wallet_operation_duration_seconds", "Wallet operation duration", duration.Seconds(),
			attribute.String("operation", "create_movement"),
			attribute.String("status", status),
		)
	}()

	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "invalid command")
	}

	// Parse wallet ID
	walletID, err := models.NewID(cmd.WalletID)
	if err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "invalid wallet ID")
	}

	// Find wallet
	wallet, err := uc.walletRepository.FindByID(ctx, walletID)
	if err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "failed to find wallet")
	}

	if wallet == nil {
		err := errors.New("wallet not found")
		span.RecordError(err)
		return nil, err
	}

	// Add wallet attributes to span
	span.SetAttributes(
		attribute.String("user_id", wallet.UserID.String()),
		attribute.Float64("balance_before", float64(wallet.Balance.Amount)/100.0),
	)

	// Create money object
	amount := models.NewMoney(cmd.Amount, cmd.Currency)

	var transaction *domain.Transaction
	var paymentID *models.ID

	// Parse payment ID if provided
	if cmd.PaymentID != "" {
		pid, err := models.NewID(cmd.PaymentID)
		if err != nil {
			return nil, errors.Wrap(err, "invalid payment ID")
		}
		paymentID = &pid
	}

	// Create movement based on type
	switch cmd.Type {
	case "income":
		// Income = Credit to wallet
		transaction, err = wallet.Credit(amount, cmd.Reference, paymentID)
		if err != nil {
			span.RecordError(err)
			return nil, errors.Wrap(err, "failed to credit wallet")
		}

	case "expense":
		// Expense = Debit from wallet
		if paymentID == nil {
			err := errors.New("payment ID is required for expense movements")
			span.RecordError(err)
			return nil, err
		}
		transaction, err = wallet.Debit(amount, *paymentID, cmd.Reference)
		if err != nil {
			span.RecordError(err)
			return nil, errors.Wrap(err, "failed to debit wallet")
		}

	default:
		err := errors.New("invalid movement type, must be 'income' or 'expense'")
		span.RecordError(err)
		return nil, err
	}

	// Save wallet
	if err := uc.walletRepository.Save(ctx, wallet); err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "failed to save wallet")
	}

	// Save transaction
	if err := uc.transactionRepository.Save(ctx, transaction); err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "failed to save transaction")
	}

	// Publish domain events with tracing
	if len(wallet.Events()) > 0 {
		uc.publishEventsWithTracing(ctx, wallet.Events())
		if err := uc.eventPublisher.Publish(ctx, wallet.Events()...); err != nil {
			span.RecordError(err)
			return nil, errors.Wrap(err, "failed to publish events")
		}
	}

	// Clear events
	wallet.ClearEvents()

	// Create movement recorded event
	movementEvent := events.NewEvent(wallet.ID, events.WalletMovementCreatedEvent, WalletMovementCreatedData{
		WalletID:      wallet.ID,
		TransactionID: transaction.ID,
		UserID:        wallet.UserID,
		Type:          cmd.Type,
		Amount:        amount,
		BalanceBefore: transaction.BalanceBefore,
		BalanceAfter:  transaction.BalanceAfter,
		Reference:     cmd.Reference,
		Description:   cmd.Description,
		PaymentID:     paymentID,
	})

	// Publish movement event with tracing
	uc.publishEventsWithTracing(ctx, []*events.Event{movementEvent})
	if err := uc.eventPublisher.Publish(ctx, movementEvent); err != nil {
		span.RecordError(err)
		return nil, errors.Wrap(err, "failed to publish movement created event")
	}

	// Record wallet balance metric
	telemetry.RecordGauge(ctx, "wallet_balance", "Current wallet balance", float64(wallet.Balance.Amount)/100.0,
		attribute.String("wallet_id", wallet.ID.String()),
		attribute.String("user_id", wallet.UserID.String()),
	)

	status = "success"
	span.SetAttributes(
		attribute.String("transaction_id", transaction.ID.String()),
		attribute.Float64("balance_after", float64(wallet.Balance.Amount)/100.0),
	)

	return &CreateMovementResponse{
		TransactionID: transaction.ID.String(),
		WalletID:      wallet.ID.String(),
		Type:          cmd.Type,
		Amount:        transaction.Amount,
		BalanceAfter:  wallet.Balance,
	}, nil
}

// validateCommand validates the create movement command
func (uc *CreateMovement) validateCommand(cmd *CreateMovementCommand) error {
	if cmd.WalletID == "" {
		return errors.New("wallet ID is required")
	}

	if cmd.Type == "" {
		return errors.New("movement type is required")
	}

	if cmd.Type != "income" && cmd.Type != "expense" {
		return errors.New("movement type must be 'income' or 'expense'")
	}

	if cmd.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if cmd.Currency == "" {
		return errors.New("currency is required")
	}

	if cmd.Reference == "" {
		return errors.New("reference is required")
	}

	return nil
}

// publishEventsWithTracing publishes events with telemetry tracking
func (uc *CreateMovement) publishEventsWithTracing(ctx context.Context, events []*events.Event) {
	for _, event := range events {
		start := time.Now()
		eventType := event.EventType

		defer func() {
			duration := time.Since(start)
			// Record event publishing metrics
			telemetry.RecordCounter(ctx, "events_published_total", "Total events published", 1,
				attribute.String("event_type", eventType),
				attribute.String("status", "success"),
			)
			telemetry.RecordHistogram(ctx, "event_publish_duration_seconds", "Event publishing duration", duration.Seconds(),
				attribute.String("event_type", eventType),
				attribute.String("status", "success"),
			)
		}()
	}
}

// WalletMovementCreatedData represents data for wallet movement created event
type WalletMovementCreatedData struct {
	WalletID      models.ID    `json:"wallet_id"`
	TransactionID models.ID    `json:"transaction_id"`
	UserID        models.ID    `json:"user_id"`
	Type          string       `json:"type"`
	Amount        models.Money `json:"amount"`
	BalanceBefore models.Money `json:"balance_before"`
	BalanceAfter  models.Money `json:"balance_after"`
	Reference     string       `json:"reference"`
	Description   string       `json:"description,omitempty"`
	PaymentID     *models.ID   `json:"payment_id,omitempty"`
}