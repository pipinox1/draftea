package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessExternalProviderUpdatesCommand represents the command to process external provider updates
type ProcessExternalProviderUpdatesCommand struct {
	Provider         string                 `json:"provider"`
	EventType        string                 `json:"event_type"`
	TransactionID    string                 `json:"transaction_id"`
	ExternalID       string                 `json:"external_id"`
	PaymentReference string                 `json:"payment_reference"`
	Amount           models.Money           `json:"amount"`
	Status           string                 `json:"status"`
	ErrorCode        string                 `json:"error_code,omitempty"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessExternalProviderUpdates use case converts external provider updates into payment operations
type ProcessExternalProviderUpdates struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessExternalProviderUpdates creates a new ProcessExternalProviderUpdates use case
func NewProcessExternalProviderUpdates(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessExternalProviderUpdates {
	return &ProcessExternalProviderUpdates{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes external provider updates and creates corresponding payment operations
func (uc *ProcessExternalProviderUpdates) Execute(ctx context.Context, cmd *ProcessExternalProviderUpdatesCommand) error {
	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		return errors.Wrap(err, "invalid command")
	}

	// Parse payment ID from payment reference
	paymentID, err := models.NewID(cmd.PaymentReference)
	if err != nil {
		return errors.Wrap(err, "invalid payment reference")
	}

	// Find payment
	payment, err := uc.paymentRepository.FindByID(ctx, paymentID)
	if err != nil {
		return errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return errors.New("payment not found")
	}

	// Verify this payment uses the correct provider
	if payment.PaymentMethod.PaymentMethodType.String() != cmd.Provider {
		return errors.New("payment method provider mismatch")
	}

	// Create payment operation based on external provider update
	var operation *domain.PaymentOperation

	switch uc.normalizeStatus(cmd.Status, cmd.EventType) {
	case "completed", "succeeded", "paid":
		// Create successful operation
		operationType := uc.getOperationType(cmd.EventType)
		operation = domain.NewPaymentOperation(
			payment.ID,
			operationType,
			cmd.Amount,
			cmd.Provider,
		)

		// Complete the operation with external transaction details
		operation.Complete(cmd.TransactionID, cmd.ExternalID)

	case "failed", "canceled", "cancelled":
		// Create failed operation
		operationType := uc.getOperationType(cmd.EventType)
		operation = domain.NewPaymentOperation(
			payment.ID,
			operationType,
			cmd.Amount,
			cmd.Provider,
		)

		// Fail the operation with error details
		errorCode := cmd.ErrorCode
		if errorCode == "" {
			errorCode = "external_provider_error"
		}
		errorMessage := cmd.ErrorMessage
		if errorMessage == "" {
			errorMessage = "Payment failed at external provider"
		}

		operation.Fail(errorCode, errorMessage)

	case "processing", "pending":
		// Create processing operation
		operationType := uc.getOperationType(cmd.EventType)
		operation = domain.NewPaymentOperation(
			payment.ID,
			operationType,
			cmd.Amount,
			cmd.Provider,
		)

		// Mark as processing
		operation.Process()

	default:
		// Unknown status, log and ignore
		return errors.Errorf("unknown external provider status: %s", cmd.Status)
	}

	// Add metadata to operation if provided
	if cmd.Metadata != nil {
		for key, value := range cmd.Metadata {
			operation.Metadata[key] = value
		}
	}

	// Publish payment operation events
	if err := uc.eventPublisher.Publish(ctx, operation.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish payment operation events")
	}

	// Clear operation events
	operation.ClearEvents()

	return nil
}

// normalizeStatus normalizes different provider statuses to common values
func (uc *ProcessExternalProviderUpdates) normalizeStatus(status, eventType string) string {
	// Normalize based on common external provider statuses
	switch status {
	case "succeeded", "success", "completed", "paid", "confirmed":
		return "completed"
	case "failed", "failure", "error", "declined":
		return "failed"
	case "canceled", "cancelled", "void":
		return "cancelled"
	case "processing", "pending", "in_progress":
		return "processing"
	default:
		// Try to infer from event type
		switch eventType {
		case "payment_intent.succeeded", "charge.succeeded":
			return "completed"
		case "payment_intent.payment_failed", "charge.failed":
			return "failed"
		case "payment_intent.canceled":
			return "cancelled"
		case "payment_intent.processing":
			return "processing"
		default:
			return status
		}
	}
}

// getOperationType determines operation type based on event type
func (uc *ProcessExternalProviderUpdates) getOperationType(eventType string) domain.PaymentOperationType {
	switch eventType {
	case "refund.created", "refund.succeeded", "refund.updated":
		return domain.PaymentOperationTypeRefund
	case "payment_intent.canceled", "charge.dispute.created":
		return domain.PaymentOperationTypeReversal
	default:
		return domain.PaymentOperationTypeDebit
	}
}

// validateCommand validates the process external provider updates command
func (uc *ProcessExternalProviderUpdates) validateCommand(cmd *ProcessExternalProviderUpdatesCommand) error {
	if cmd.Provider == "" {
		return errors.New("provider is required")
	}

	if cmd.EventType == "" {
		return errors.New("event type is required")
	}

	if cmd.PaymentReference == "" {
		return errors.New("payment reference is required")
	}

	if cmd.Amount.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if cmd.Status == "" {
		return errors.New("status is required")
	}

	return nil
}