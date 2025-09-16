package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessPaymentInconsistentOperationCommand represents the command to process inconsistent payments
type ProcessPaymentInconsistentOperationCommand struct {
	PaymentID    models.ID `json:"payment_id"`
	Reason       string    `json:"reason"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
}

// ProcessPaymentInconsistentOperation use case handles inconsistent payments by refunding and rolling back operations
type ProcessPaymentInconsistentOperation struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessPaymentInconsistentOperation creates a new ProcessPaymentInconsistentOperation use case
func NewProcessPaymentInconsistentOperation(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessPaymentInconsistentOperation {
	return &ProcessPaymentInconsistentOperation{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes inconsistent payments by initiating compensating actions
func (uc *ProcessPaymentInconsistentOperation) Execute(ctx context.Context, cmd *ProcessPaymentInconsistentOperationCommand) error {
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

	// Log the inconsistent state for audit purposes
	auditEvent := events.NewEvent(payment.ID, events.PaymentInconsistentOperationStartedEvent, PaymentInconsistentOperationStartedData{
		PaymentID:     payment.ID,
		PaymentStatus: payment.Status,
		Reason:        cmd.Reason,
		ErrorCode:     cmd.ErrorCode,
		ErrorMessage:  cmd.ErrorMessage,
	})

	if err := uc.eventPublisher.Publish(ctx, auditEvent); err != nil {
		return errors.Wrap(err, "failed to publish audit event")
	}

	// Determine compensating actions based on payment status and reason
	switch payment.Status {
	case domain.PaymentStatusCompleted:
		// Payment was completed but there's an inconsistency - initiate full refund
		err = uc.initiateFullRefund(ctx, payment, cmd.Reason)

	case domain.PaymentStatusProcessing:
		// Payment is in processing state - try to cancel first, then refund if needed
		err = uc.initiateCancellationOrRefund(ctx, payment, cmd.Reason)

	case domain.PaymentStatusFailed:
		// Payment already failed - check if wallet was debited and needs credit back
		err = uc.initiateWalletCredit(ctx, payment, cmd.Reason)

	default:
		// For other statuses, mark as failed
		err = payment.Fail(cmd.ErrorMessage, cmd.ErrorCode)
		if err == nil {
			err = uc.paymentRepository.Save(ctx, payment)
		}
	}

	if err != nil {
		return errors.Wrap(err, "failed to initiate compensating action")
	}

	// Publish completion event
	completionEvent := events.NewEvent(payment.ID, events.PaymentInconsistentOperationProcessedEvent, PaymentInconsistentOperationProcessedData{
		PaymentID: payment.ID,
		Reason:    cmd.Reason,
		Action:    uc.getCompensatingAction(payment.Status),
	})

	if err := uc.eventPublisher.Publish(ctx, completionEvent); err != nil {
		return errors.Wrap(err, "failed to publish completion event")
	}

	return nil
}

// initiateFullRefund initiates a full refund for a completed payment
func (uc *ProcessPaymentInconsistentOperation) initiateFullRefund(ctx context.Context, payment *domain.Payment, reason string) error {
	// Create refund operation based on payment method
	switch payment.PaymentMethod.PaymentMethodType {
	case "wallet":
		// For wallet payments, initiate wallet credit
		creditEvent := events.NewEvent(payment.ID, events.WalletCreditRequestedEvent, WalletCreditRequestedData{
			PaymentID: payment.ID,
			WalletID:  payment.PaymentMethod.WalletID,
			UserID:    payment.UserID,
			Amount:    payment.Amount,
			Reference: "Refund for inconsistent payment " + payment.ID.String(),
			Reason:    reason,
		})

		return uc.eventPublisher.Publish(ctx, creditEvent)

	case "stripe", "external_gateway":
		// For external payments, create refund operation
		refundOperation := domain.NewPaymentOperation(
			payment.ID,
			domain.PaymentOperationTypeRefund,
			payment.Amount,
			payment.PaymentMethod.PaymentMethodType.String(),
		)

		// Publish operation events - external service will handle the actual refund
		return uc.eventPublisher.Publish(ctx, refundOperation.Events()...)

	default:
		return errors.New("unsupported payment method for refund")
	}
}

// initiateCancellationOrRefund tries to cancel or refund a processing payment
func (uc *ProcessPaymentInconsistentOperation) initiateCancellationOrRefund(ctx context.Context, payment *domain.Payment, reason string) error {
	// First try to cancel the payment
	if err := payment.Cancel(); err != nil {
		return errors.Wrap(err, "failed to cancel payment")
	}

	// Save the cancelled payment
	if err := uc.paymentRepository.Save(ctx, payment); err != nil {
		return errors.Wrap(err, "failed to save cancelled payment")
	}

	// Publish payment events
	if err := uc.eventPublisher.Publish(ctx, payment.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish payment events")
	}

	payment.ClearEvents()

	// Also initiate refund in case money was already captured
	return uc.initiateFullRefund(ctx, payment, reason)
}

// initiateWalletCredit initiates wallet credit for failed payments where wallet might have been debited
func (uc *ProcessPaymentInconsistentOperation) initiateWalletCredit(ctx context.Context, payment *domain.Payment, reason string) error {
	if payment.PaymentMethod.PaymentMethodType != "wallet" {
		// For non-wallet payments, no action needed
		return nil
	}

	// Credit the wallet back
	creditEvent := events.NewEvent(payment.ID, events.WalletCreditRequestedEvent, WalletCreditRequestedData{
		PaymentID: payment.ID,
		WalletID:  payment.PaymentMethod.WalletID,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		Reference: "Credit for failed inconsistent payment " + payment.ID.String(),
		Reason:    reason,
	})

	return uc.eventPublisher.Publish(ctx, creditEvent)
}

// getCompensatingAction returns the compensating action taken based on payment status
func (uc *ProcessPaymentInconsistentOperation) getCompensatingAction(status domain.PaymentStatus) string {
	switch status {
	case domain.PaymentStatusCompleted:
		return "full_refund_initiated"
	case domain.PaymentStatusProcessing:
		return "cancellation_and_refund_initiated"
	case domain.PaymentStatusFailed:
		return "wallet_credit_initiated"
	default:
		return "payment_marked_failed"
	}
}

// validateCommand validates the process payment inconsistent operation command
func (uc *ProcessPaymentInconsistentOperation) validateCommand(cmd *ProcessPaymentInconsistentOperationCommand) error {
	if cmd.PaymentID.String() == "" {
		return errors.New("payment ID is required")
	}

	if cmd.Reason == "" {
		return errors.New("reason is required")
	}

	return nil
}

// Event Data Structures
type PaymentInconsistentOperationStartedData struct {
	PaymentID     models.ID             `json:"payment_id"`
	PaymentStatus domain.PaymentStatus  `json:"payment_status"`
	Reason        string                `json:"reason"`
	ErrorCode     string                `json:"error_code"`
	ErrorMessage  string                `json:"error_message"`
}

type PaymentInconsistentOperationProcessedData struct {
	PaymentID models.ID `json:"payment_id"`
	Reason    string    `json:"reason"`
	Action    string    `json:"action"`
}

type WalletCreditRequestedData struct {
	PaymentID models.ID    `json:"payment_id"`
	WalletID  string       `json:"wallet_id"`
	UserID    models.ID    `json:"user_id"`
	Amount    models.Money `json:"amount"`
	Reference string       `json:"reference"`
	Reason    string       `json:"reason"`
}