package domain

import (
	"context"
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusInitiated  PaymentStatus = "initiated"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusCancelled  PaymentStatus = "cancelled"
)

// Payment aggregate root
type Payment struct {
	ID            models.ID
	UserID        models.ID
	Amount        models.Money
	PaymentMethod PaymentMethod
	Description   string
	Status        PaymentStatus
	Timestamps    models.Timestamps
	Version       models.Version

	events []*events.Event
}

// CreatePayment factory method
func CreatePayment(userID models.ID, amount models.Money, paymentMethod PaymentMethod, description string) (*Payment, error) {
	if !amount.IsPositive() {
		return nil, errors.New("amount must be positive")
	}

	payment := &Payment{
		ID:            models.GenerateUUID(),
		UserID:        userID,
		Amount:        amount,
		PaymentMethod: paymentMethod,
		Description:   description,
		Status:        PaymentStatusInitiated,
		Timestamps:    models.NewTimestamps(),
		Version:       models.NewVersion(),
	}

	// Record domain event
	event := events.NewEvent(payment.ID, events.PaymentCreatedEvent, PaymentInitiatedData{
		PaymentID:     payment.ID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		PaymentMethod: payment.PaymentMethod,
		Description:   payment.Description,
	})

	payment.recordEvent(event)
	return payment, nil
}

// Process marks payment as processing
func (p *Payment) Process() error {
	if p.Status != PaymentStatusInitiated {
		return errors.New("payment can only be processed from initiated status")
	}

	p.Status = PaymentStatusProcessing
	p.Timestamps = p.Timestamps.Update()
	p.Version = p.Version.Update()

	event := events.NewEvent(p.ID, events.PaymentProcessingEvent, PaymentProcessingData{
		PaymentID: p.ID,
		UserID:    p.UserID,
	})

	p.recordEvent(event)
	return nil
}

// Complete marks payment as completed
func (p *Payment) Complete(gatewayTransactionID, transactionID string) error {
	if p.Status != PaymentStatusProcessing {
		return errors.New("payment can only be completed from processing status")
	}

	p.Status = PaymentStatusCompleted
	p.Timestamps = p.Timestamps.Update()
	p.Version = p.Version.Update()

	event := events.NewEvent(p.ID, events.PaymentCompletedEvent, PaymentCompletedData{
		PaymentID:            p.ID,
		UserID:               p.UserID,
		Amount:               p.Amount,
		GatewayTransactionID: gatewayTransactionID,
		TransactionID:        transactionID,
		CompletedAt:          time.Now(),
	})

	p.recordEvent(event)
	return nil
}

// Fail marks payment as failed
func (p *Payment) Fail(reason string, errorCode string) error {
	if p.Status == PaymentStatusCompleted {
		return errors.New("cannot fail a completed payment")
	}

	p.Status = PaymentStatusFailed
	p.Timestamps = p.Timestamps.Update()
	p.Version = p.Version.Update()

	event := events.NewEvent(p.ID, events.PaymentFailedEvent, PaymentFailedData{
		PaymentID: p.ID,
		UserID:    p.UserID,
		Amount:    p.Amount,
		Reason:    reason,
		ErrorCode: errorCode,
		FailedAt:  time.Now(),
	})

	p.recordEvent(event)
	return nil
}

// Cancel marks payment as cancelled
func (p *Payment) Cancel() error {
	if p.Status == PaymentStatusCompleted {
		return errors.New("cannot cancel a completed payment")
	}

	p.Status = PaymentStatusCancelled
	p.Timestamps = p.Timestamps.Update()
	p.Version = p.Version.Update()

	event := events.NewEvent(p.ID, events.PaymentCancelledEvent, PaymentCancelledData{
		PaymentID:   p.ID,
		UserID:      p.UserID,
		CancelledAt: time.Now(),
	})

	p.recordEvent(event)
	return nil
}

// Events returns domain events
func (p *Payment) Events() []*events.Event {
	return p.events
}

// ClearEvents clears domain events
func (p *Payment) ClearEvents() {
	p.events = make([]*events.Event, 0)
}

// recordEvent records a domain event
func (p *Payment) recordEvent(event *events.Event) {
	p.events = append(p.events, event)
}

// Event Data Structures
type PaymentInitiatedData struct {
	PaymentID     models.ID     `json:"payment_id"`
	UserID        models.ID     `json:"user_id"`
	Amount        models.Money  `json:"amount"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	Description   string        `json:"description"`
}

type PaymentProcessingData struct {
	PaymentID models.ID `json:"payment_id"`
	UserID    models.ID `json:"user_id"`
}

type PaymentCompletedData struct {
	PaymentID            models.ID    `json:"payment_id"`
	UserID               models.ID    `json:"user_id"`
	Amount               models.Money `json:"amount"`
	GatewayTransactionID string       `json:"gateway_transaction_id"`
	TransactionID        string       `json:"transaction_id"`
	CompletedAt          time.Time    `json:"completed_at"`
}

type PaymentFailedData struct {
	PaymentID models.ID    `json:"payment_id"`
	UserID    models.ID    `json:"user_id"`
	Amount    models.Money `json:"amount"`
	Reason    string       `json:"reason"`
	ErrorCode string       `json:"error_code"`
	FailedAt  time.Time    `json:"failed_at"`
}

type PaymentCancelledData struct {
	PaymentID   models.ID `json:"payment_id"`
	UserID      models.ID `json:"user_id"`
	CancelledAt time.Time `json:"cancelled_at"`
}

// PaymentRepository interface
type PaymentRepository interface {
	Save(ctx context.Context, payment *Payment) error
	FindByID(ctx context.Context, id models.ID) (*Payment, error)
	FindByUserID(ctx context.Context, userID models.ID) ([]*Payment, error)
}
