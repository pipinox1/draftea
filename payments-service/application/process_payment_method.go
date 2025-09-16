package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessPaymentMethodCommand represents the command to process payment method
type ProcessPaymentMethodCommand struct {
	PaymentID models.ID `json:"payment_id"`
}

// ProcessPaymentMethod use case handles processing payment based on payment method
type ProcessPaymentMethod struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessPaymentMethod creates a new ProcessPaymentMethod use case
func NewProcessPaymentMethod(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessPaymentMethod {
	return &ProcessPaymentMethod{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes payment method based on payment type
func (uc *ProcessPaymentMethod) Execute(ctx context.Context, cmd *ProcessPaymentMethodCommand) error {
	// Find payment
	payment, err := uc.paymentRepository.FindByID(ctx, cmd.PaymentID)
	if err != nil {
		return errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return errors.New("payment not found")
	}

	// Only process initiated payments
	if payment.Status != domain.PaymentStatusInitiated {
		return errors.New("payment must be in initiated status to process")
	}

	// Mark payment as processing
	if err := payment.Process(); err != nil {
		return errors.Wrap(err, "failed to mark payment as processing")
	}

	// Save updated payment
	if err := uc.paymentRepository.Save(ctx, payment); err != nil {
		return errors.Wrap(err, "failed to save payment")
	}

	// Process based on payment method type
	switch payment.PaymentMethod.PaymentMethodType {
	case domain.PaymentMethodTypeWallet:
		// For wallet payments, request wallet debit
		debitEvent := events.NewEvent(payment.ID, events.WalletDebitRequestedEvent, WalletDebitRequestedData{
			PaymentID: payment.ID,
			WalletID:  payment.PaymentMethod.WalletPaymentMethod.WalletID,
			UserID:    payment.UserID,
			Amount:    payment.Amount,
			Reference: "Payment " + payment.ID.String(),
		})

		if err := uc.eventPublisher.Publish(ctx, debitEvent); err != nil {
			return errors.Wrap(err, "failed to publish wallet debit requested event")
		}

	case domain.PaymentMethodTypeCreditCard:
		// For external payment methods, create a payment operation for external processing
		operation := domain.NewPaymentOperation(
			payment.ID,
			domain.PaymentOperationTypeDebit,
			payment.Amount,
			payment.PaymentMethod.PaymentMethodType.String(),
		)

		// Publish operation created event - external service will handle this
		if err := uc.eventPublisher.Publish(ctx, operation.Events()...); err != nil {
			return errors.Wrap(err, "failed to publish payment operation events")
		}

	default:
		// Mark payment as failed for unsupported payment methods
		if err := payment.Fail("unsupported_payment_method", "Payment method not supported"); err != nil {
			return errors.Wrap(err, "failed to mark payment as failed")
		}

		if err := uc.paymentRepository.Save(ctx, payment); err != nil {
			return errors.Wrap(err, "failed to save failed payment")
		}
	}

	// Publish payment events
	if err := uc.eventPublisher.Publish(ctx, payment.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish payment events")
	}

	// Clear events
	payment.ClearEvents()

	return nil
}

// WalletDebitRequestedData represents data for wallet debit request event
type WalletDebitRequestedData struct {
	PaymentID models.ID    `json:"payment_id"`
	WalletID  string       `json:"wallet_id"`
	UserID    models.ID    `json:"user_id"`
	Amount    models.Money `json:"amount"`
	Reference string       `json:"reference"`
}
