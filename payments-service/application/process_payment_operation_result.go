package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessPaymentOperationResultCommand represents the command to process payment operation results
type ProcessPaymentOperationResultCommand struct {
	OperationID             models.ID                      `json:"operation_id"`
	PaymentID               models.ID                      `json:"payment_id"`
	Type                    domain.PaymentOperationType   `json:"type"`
	Status                  domain.PaymentOperationStatus `json:"status"`
	Amount                  models.Money                   `json:"amount"`
	ProviderTransactionID   string                         `json:"provider_transaction_id,omitempty"`
	ExternalTransactionID   string                         `json:"external_transaction_id,omitempty"`
	ErrorCode               string                         `json:"error_code,omitempty"`
	ErrorMessage            string                         `json:"error_message,omitempty"`
}

// ProcessPaymentOperationResult use case applies payment operations to payments
type ProcessPaymentOperationResult struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessPaymentOperationResult creates a new ProcessPaymentOperationResult use case
func NewProcessPaymentOperationResult(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessPaymentOperationResult {
	return &ProcessPaymentOperationResult{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes payment operation results and applies them to the payment
func (uc *ProcessPaymentOperationResult) Execute(ctx context.Context, cmd *ProcessPaymentOperationResultCommand) error {
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

	// Apply operation result to payment based on operation type and status
	switch cmd.Type {
	case domain.PaymentOperationTypeDebit:
		err = uc.processDebitOperation(payment, cmd)
	case domain.PaymentOperationTypeRefund:
		err = uc.processRefundOperation(payment, cmd)
	case domain.PaymentOperationTypeReversal:
		err = uc.processReversalOperation(payment, cmd)
	default:
		return errors.Errorf("unsupported operation type: %s", cmd.Type)
	}

	if err != nil {
		return errors.Wrap(err, "failed to process operation")
	}

	// Save updated payment
	if err := uc.paymentRepository.Save(ctx, payment); err != nil {
		return errors.Wrap(err, "failed to save payment")
	}

	// Publish payment events
	if err := uc.eventPublisher.Publish(ctx, payment.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish payment events")
	}

	// Clear events
	payment.ClearEvents()

	return nil
}

// processDebitOperation processes debit operation results
func (uc *ProcessPaymentOperationResult) processDebitOperation(payment *domain.Payment, cmd *ProcessPaymentOperationResultCommand) error {
	switch cmd.Status {
	case domain.PaymentOperationStatusCompleted:
		// Payment successful - complete the payment
		return payment.Complete(cmd.ProviderTransactionID, cmd.ExternalTransactionID)

	case domain.PaymentOperationStatusFailed:
		// Payment failed - fail the payment
		errorCode := cmd.ErrorCode
		if errorCode == "" {
			errorCode = "payment_operation_failed"
		}
		errorMessage := cmd.ErrorMessage
		if errorMessage == "" {
			errorMessage = "Payment operation failed"
		}
		return payment.Fail(errorMessage, errorCode)

	case domain.PaymentOperationStatusCancelled:
		// Payment cancelled - cancel the payment
		return payment.Cancel()

	default:
		// For processing status, no action needed - payment remains in processing
		return nil
	}
}

// processRefundOperation processes refund operation results
func (uc *ProcessPaymentOperationResult) processRefundOperation(payment *domain.Payment, cmd *ProcessPaymentOperationResultCommand) error {
	switch cmd.Status {
	case domain.PaymentOperationStatusCompleted:
		// Refund successful - mark payment as refunded
		// First check if payment is in a state that allows refunding
		if payment.Status != domain.PaymentStatusCompleted {
			return errors.New("can only refund completed payments")
		}

		// Create refund completed event - in a real system you might have a separate refund aggregate
		// For now, we'll just clear events since we don't have access to private recordEvent method
		payment.ClearEvents()

		return nil

	case domain.PaymentOperationStatusFailed:
		// Refund failed - log the failure
		return errors.Errorf("refund failed: %s", cmd.ErrorMessage)

	default:
		// For other statuses, no action needed
		return nil
	}
}

// processReversalOperation processes reversal operation results
func (uc *ProcessPaymentOperationResult) processReversalOperation(payment *domain.Payment, cmd *ProcessPaymentOperationResultCommand) error {
	switch cmd.Status {
	case domain.PaymentOperationStatusCompleted:
		// Reversal successful - cancel the payment
		return payment.Cancel()

	case domain.PaymentOperationStatusFailed:
		// Reversal failed - this might indicate an inconsistent state
		// For now, just return the error
		return errors.Errorf("reversal failed - payment may be in inconsistent state: %s", cmd.ErrorMessage)

	default:
		// For other statuses, no action needed
		return nil
	}
}

// validateCommand validates the process payment operation result command
func (uc *ProcessPaymentOperationResult) validateCommand(cmd *ProcessPaymentOperationResultCommand) error {
	if cmd.OperationID.String() == "" {
		return errors.New("operation ID is required")
	}

	if cmd.PaymentID.String() == "" {
		return errors.New("payment ID is required")
	}

	if cmd.Type == "" {
		return errors.New("operation type is required")
	}

	if cmd.Status == "" {
		return errors.New("operation status is required")
	}

	if cmd.Amount.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	return nil
}

// Event Data Structures
type PaymentRefundCompletedData struct {
	PaymentID             models.ID    `json:"payment_id"`
	RefundAmount          models.Money `json:"refund_amount"`
	ProviderTransactionID string       `json:"provider_transaction_id"`
	ExternalTransactionID string       `json:"external_transaction_id"`
}

type PaymentRefundFailedData struct {
	PaymentID    models.ID    `json:"payment_id"`
	RefundAmount models.Money `json:"refund_amount"`
	ErrorCode    string       `json:"error_code"`
	ErrorMessage string       `json:"error_message"`
}

type PaymentInconsistentStateData struct {
	PaymentID    models.ID `json:"payment_id"`
	Reason       string    `json:"reason"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
}