package application

import (
	"context"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// GetPaymentQuery represents the query to get a payment
type GetPaymentQuery struct {
	PaymentID string `json:"payment_id"`
}

// GetPaymentResponse represents the response for getting a payment
type GetPaymentResponse struct {
	PaymentID     string               `json:"payment_id"`
	UserID        string               `json:"user_id"`
	Amount        int64                `json:"amount"`
	Currency      string               `json:"currency"`
	PaymentMethod domain.PaymentMethod `json:"payment_method"`
	Description   string               `json:"description"`
	Status        string               `json:"status"`
	CreatedAt     string               `json:"created_at"`
	UpdatedAt     string               `json:"updated_at"`
}

// GetPayment use case
type GetPayment struct {
	paymentRepository domain.PaymentRepository
}

// NewGetPayment creates a new GetPayment use case
func NewGetPayment(paymentRepository domain.PaymentRepository) *GetPayment {
	return &GetPayment{
		paymentRepository: paymentRepository,
	}
}

// Execute executes the get payment use case
func (uc *GetPayment) Execute(ctx context.Context, query *GetPaymentQuery) (*GetPaymentResponse, error) {
	if query.PaymentID == "" {
		return nil, errors.New("payment ID is required")
	}

	paymentID, err := models.NewID(query.PaymentID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid payment ID")
	}

	payment, err := uc.paymentRepository.FindByID(ctx, paymentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find payment")
	}

	if payment == nil {
		return nil, errors.New("payment not found")
	}
	
	response := &GetPaymentResponse{
		PaymentID:     payment.ID.String(),
		UserID:        payment.UserID.String(),
		Amount:        payment.Amount.Amount,
		Currency:      payment.Amount.Currency,
		PaymentMethod: payment.PaymentMethod,
		Description:   payment.Description,
		Status:        string(payment.Status),
		CreatedAt:     payment.Timestamps.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     payment.Timestamps.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return response, nil
}
