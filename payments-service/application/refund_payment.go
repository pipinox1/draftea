package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// RefundPaymentCommand represents the command to refund a payment
type RefundPaymentCommand struct {
	PaymentID   models.ID    `json:"payment_id"`
	Amount      models.Money `json:"amount,omitempty"` // Optional for partial refunds
	Reason      string       `json:"reason"`
	RequestedBy models.ID    `json:"requested_by"`
}

// RefundPaymentResponse represents the response after initiating a refund
type RefundPaymentResponse struct {
	PaymentID models.ID `json:"payment_id"`
	RefundID  models.ID `json:"refund_id"`
	Amount    models.Money `json:"amount"`
	Status    string    `json:"status"`
}

// RefundPayment use case publishes refund initiation event to begin the refund process
type RefundPayment struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewRefundPayment creates a new RefundPayment use case
func NewRefundPayment(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *RefundPayment {
	return &RefundPayment{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute initiates the refund process for a payment
func (uc *RefundPayment) Execute(ctx context.Context, cmd *RefundPaymentCommand) (*RefundPaymentResponse, error) {
	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		return nil, errors.Wrap(err, "invalid command")
	}

	// Find payment
	payment, err := uc.paymentRepository.FindByID(ctx, cmd.PaymentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return nil, errors.New("payment not found")
	}

	// Validate that payment can be refunded
	if err := uc.validateRefundEligibility(payment, cmd.Amount); err != nil {
		return nil, errors.Wrap(err, "payment not eligible for refund")
	}

	// Determine refund amount
	refundAmount := cmd.Amount
	if refundAmount.Amount == 0 {
		// Full refund if no amount specified
		refundAmount = payment.Amount
	}

	// Generate refund ID
	refundID := models.GenerateUUID()

	// Publish refund initiated event - this will trigger the refund saga
	refundEvent := events.NewEvent(payment.ID, events.PaymentRefundInitiatedEvent, PaymentRefundInitiatedData{
		PaymentID:   payment.ID,
		RefundID:    refundID,
		Amount:      refundAmount,
		Reason:      cmd.Reason,
		RequestedBy: cmd.RequestedBy,
		PaymentMethod: payment.PaymentMethod,
		UserID:      payment.UserID,
	})

	if err := uc.eventPublisher.Publish(ctx, refundEvent); err != nil {
		return nil, errors.Wrap(err, "failed to publish refund initiated event")
	}

	return &RefundPaymentResponse{
		PaymentID: payment.ID,
		RefundID:  refundID,
		Amount:    refundAmount,
		Status:    "initiated",
	}, nil
}

// validateRefundEligibility checks if a payment can be refunded
func (uc *RefundPayment) validateRefundEligibility(payment *domain.Payment, refundAmount models.Money) error {
	// Only completed payments can be refunded
	if payment.Status != domain.PaymentStatusCompleted {
		return errors.New("only completed payments can be refunded")
	}

	// If partial refund amount is specified, validate it
	if refundAmount.Amount != 0 {
		if refundAmount.Amount <= 0 {
			return errors.New("refund amount must be positive")
		}

		if refundAmount.Currency != payment.Amount.Currency {
			return errors.New("refund currency must match payment currency")
		}

		if refundAmount.Amount > payment.Amount.Amount {
			return errors.New("refund amount cannot exceed payment amount")
		}
	}

	// TODO: In a real system, you might also check:
	// - If the payment has already been partially or fully refunded
	// - Time-based refund policies
	// - Merchant/business specific refund rules

	return nil
}

// validateCommand validates the refund payment command
func (uc *RefundPayment) validateCommand(cmd *RefundPaymentCommand) error {
	if cmd.PaymentID.String() == "" {
		return errors.New("payment ID is required")
	}

	if cmd.Reason == "" {
		return errors.New("reason is required")
	}

	if cmd.RequestedBy.String() == "" {
		return errors.New("requested by user ID is required")
	}

	// If amount is specified, validate it
	if cmd.Amount.Amount > 0 {
		if cmd.Amount.Currency == "" {
			return errors.New("currency is required when amount is specified")
		}
	}

	return nil
}

// PaymentRefundInitiatedData represents data for payment refund initiated event
type PaymentRefundInitiatedData struct {
	PaymentID     models.ID     `json:"payment_id"`
	RefundID      models.ID     `json:"refund_id"`
	Amount        models.Money  `json:"amount"`
	Reason        string        `json:"reason"`
	RequestedBy   models.ID     `json:"requested_by"`
	PaymentMethod domain.PaymentMethod `json:"payment_method"`
	UserID        models.ID     `json:"user_id"`
}