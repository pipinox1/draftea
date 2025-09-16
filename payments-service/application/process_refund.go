package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ProcessRefundCommand represents the command to process a refund
type ProcessRefundCommand struct {
	PaymentID     models.ID            `json:"payment_id"`
	RefundID      models.ID            `json:"refund_id"`
	Amount        models.Money         `json:"amount"`
	Reason        string               `json:"reason"`
	RequestedBy   models.ID            `json:"requested_by"`
	PaymentMethod domain.PaymentMethod `json:"payment_method"`
	UserID        models.ID            `json:"user_id"`
}

// ProcessRefund use case receives refund events and routes them to appropriate providers
type ProcessRefund struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewProcessRefund creates a new ProcessRefund use case
func NewProcessRefund(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *ProcessRefund {
	return &ProcessRefund{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute processes refund based on the payment method type
func (uc *ProcessRefund) Execute(ctx context.Context, cmd *ProcessRefundCommand) error {
	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		return errors.Wrap(err, "invalid command")
	}

	// Find payment to validate it exists and get additional context
	payment, err := uc.paymentRepository.FindByID(ctx, cmd.PaymentID)
	if err != nil {
		return errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return errors.New("payment not found")
	}

	// Process refund based on payment method
	switch payment.PaymentMethod.PaymentMethodType {
	case domain.PaymentMethodTypeWallet:
		return uc.processWalletRefund(ctx, cmd)
	case domain.PaymentMethodTypeDebit:
		return uc.processExternalRefund(ctx, cmd)
	default:
		return errors.Errorf("unsupported payment method for refund: %s", cmd.PaymentMethod.PaymentMethodType)
	}
}

// processWalletRefund processes refund for wallet payments
func (uc *ProcessRefund) processWalletRefund(ctx context.Context, cmd *ProcessRefundCommand) error {
	// For wallet refunds, credit the user's wallet
	creditEvent := events.NewEvent(cmd.PaymentID, events.WalletCreditRequestedEvent, WalletCreditRequestedForRefundData{
		PaymentID: cmd.PaymentID,
		RefundID:  cmd.RefundID,
		WalletID:  cmd.PaymentMethod.WalletPaymentMethod.WalletID,
		UserID:    cmd.UserID,
		Amount:    cmd.Amount,
		Reference: "Refund for payment " + cmd.PaymentID.String(),
		Reason:    cmd.Reason,
	})

	if err := uc.eventPublisher.Publish(ctx, creditEvent); err != nil {
		return errors.Wrap(err, "failed to publish wallet credit requested event")
	}

	return nil
}

// processExternalRefund processes refund for external payment providers
func (uc *ProcessRefund) processExternalRefund(ctx context.Context, cmd *ProcessRefundCommand) error {
	// For external providers, create a refund operation
	refundOperation := domain.NewPaymentOperation(
		cmd.PaymentID,
		domain.PaymentOperationTypeRefund,
		cmd.Amount,
		cmd.PaymentMethod.PaymentMethodType.String(),
	)

	// Add refund-specific metadata
	refundOperation.Metadata["refund_id"] = cmd.RefundID.String()
	refundOperation.Metadata["refund_reason"] = cmd.Reason
	refundOperation.Metadata["requested_by"] = cmd.RequestedBy.String()

	// Mark as processing since it will be handled by external service
	refundOperation.Process()

	// Publish payment operation events - external payment processor will handle these
	if err := uc.eventPublisher.Publish(ctx, refundOperation.Events()...); err != nil {
		return errors.Wrap(err, "failed to publish refund operation events")
	}

	// Clear operation events
	refundOperation.ClearEvents()

	return nil
}

// validateCommand validates the process refund command
func (uc *ProcessRefund) validateCommand(cmd *ProcessRefundCommand) error {
	if cmd.PaymentID.String() == "" {
		return errors.New("payment ID is required")
	}

	if cmd.RefundID.String() == "" {
		return errors.New("refund ID is required")
	}

	if cmd.Amount.Amount <= 0 {
		return errors.New("refund amount must be positive")
	}

	if cmd.Amount.Currency == "" {
		return errors.New("refund currency is required")
	}

	if cmd.PaymentMethod.PaymentMethodType == "" {
		return errors.New("payment method type is required")
	}

	if cmd.PaymentMethod.PaymentMethodType == "wallet" && (cmd.PaymentMethod.WalletPaymentMethod == nil || cmd.PaymentMethod.WalletPaymentMethod.WalletID == "") {
		return errors.New("wallet ID is required for wallet refunds")
	}

	if cmd.UserID.String() == "" {
		return errors.New("user ID is required")
	}

	if cmd.Reason == "" {
		return errors.New("refund reason is required")
	}

	if cmd.RequestedBy.String() == "" {
		return errors.New("requested by user ID is required")
	}

	return nil
}

// WalletCreditRequestedForRefundData represents data for wallet credit request due to refund
type WalletCreditRequestedForRefundData struct {
	PaymentID models.ID    `json:"payment_id"`
	RefundID  models.ID    `json:"refund_id"`
	WalletID  string       `json:"wallet_id"`
	UserID    models.ID    `json:"user_id"`
	Amount    models.Money `json:"amount"`
	Reference string       `json:"reference"`
	Reason    string       `json:"reason"`
}
