package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// CreatePaymentCommand represents the command to create a payment
type CreatePaymentCommand struct {
	UserID            string                 `json:"user_id"`
	Amount            int64                  `json:"amount"`
	Currency          string                 `json:"currency"`
	PaymentMethodType string                 `json:"payment_method_type"`
	WalletID          *string                `json:"wallet_id,omitempty"`
	CardToken         *string                `json:"card_token,omitempty"`
	Description       string                 `json:"description"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// CreatePaymentResponse represents the response after creating a payment
type CreatePaymentResponse struct {
	PaymentID string `json:"payment_id"`
}

// CreatePaymentChoreography use case for choreography-based saga
type CreatePaymentChoreography struct {
	paymentRepository domain.PaymentRepository
	eventPublisher    events.Publisher
}

// NewCreatePaymentChoreography creates a new CreatePaymentChoreography use case
func NewCreatePaymentChoreography(
	paymentRepository domain.PaymentRepository,
	eventPublisher events.Publisher,
) *CreatePaymentChoreography {
	return &CreatePaymentChoreography{
		paymentRepository: paymentRepository,
		eventPublisher:    eventPublisher,
	}
}

// Execute executes the create payment use case using choreography pattern
func (uc *CreatePaymentChoreography) Execute(ctx context.Context, cmd *CreatePaymentCommand) (*CreatePaymentResponse, error) {
	if err := uc.validateCommand(cmd); err != nil {
		return nil, errors.Wrap(err, "invalid command")
	}

	userID, err := models.NewID(cmd.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID")
	}

	amount := models.NewMoney(cmd.Amount, cmd.Currency)

	// Create PaymentMethodCreator from command
	creator := &domain.PaymentMethodCreator{
		WalletID:  cmd.WalletID,
		CardToken: cmd.CardToken,
	}

	// Parse payment method type
	paymentMethodType, err := domain.NewPaymentMethodType(cmd.PaymentMethodType)
	if err != nil {
		return nil, errors.Wrap(err, "invalid payment method type")
	}

	// Create PaymentMethod using factory
	paymentMethod, err := domain.NewPaymentMethod(*paymentMethodType, creator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create payment method")
	}

	payment, err := domain.CreatePayment(userID, amount, *paymentMethod, cmd.Description)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create payment")
	}

	if err := uc.paymentRepository.Save(ctx, payment); err != nil {
		return nil, errors.Wrap(err, "failed to save payment")
	}

	if err := uc.eventPublisher.Publish(ctx, payment.Events()...); err != nil {
		return nil, errors.Wrap(err, "failed to publish events")
	}

	return &CreatePaymentResponse{
		PaymentID: payment.ID.String(),
	}, nil
}

// validateCommand validates the create payment command
func (uc *CreatePaymentChoreography) validateCommand(cmd *CreatePaymentCommand) error {
	if cmd.UserID == "" {
		return errors.New("user ID is required")
	}

	if cmd.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if cmd.Currency == "" {
		return errors.New("currency is required")
	}

	if cmd.PaymentMethodType == "" {
		return errors.New("payment method type is required")
	}

	// Validate payment method type exists
	if _, err := domain.NewPaymentMethodType(cmd.PaymentMethodType); err != nil {
		return errors.Wrap(err, "invalid payment method type")
	}

	// Validate required fields based on payment method type
	switch cmd.PaymentMethodType {
	case "wallet":
		if cmd.WalletID == nil || *cmd.WalletID == "" {
			return errors.New("wallet ID is required for wallet payments")
		}
	case "credit_card", "debit":
		if cmd.CardToken == nil || *cmd.CardToken == "" {
			return errors.New("card token is required for card payments")
		}
	}

	return nil
}
