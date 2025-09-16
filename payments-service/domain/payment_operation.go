package domain

import (
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
)

// PaymentOperationType represents the type of payment operation
type PaymentOperationType string

const (
	PaymentOperationTypeDebit     PaymentOperationType = "debit"
	PaymentOperationTypeCredit    PaymentOperationType = "credit"
	PaymentOperationTypeRefund    PaymentOperationType = "refund"
	PaymentOperationTypeReversal  PaymentOperationType = "reversal"
)

// PaymentOperationStatus represents the status of a payment operation
type PaymentOperationStatus string

const (
	PaymentOperationStatusPending    PaymentOperationStatus = "pending"
	PaymentOperationStatusProcessing PaymentOperationStatus = "processing"
	PaymentOperationStatusCompleted  PaymentOperationStatus = "completed"
	PaymentOperationStatusFailed     PaymentOperationStatus = "failed"
	PaymentOperationStatusCancelled  PaymentOperationStatus = "cancelled"
)

// PaymentOperation represents an operation performed on a payment
type PaymentOperation struct {
	ID                      models.ID                  `json:"id"`
	PaymentID               models.ID                  `json:"payment_id"`
	Type                    PaymentOperationType       `json:"type"`
	Status                  PaymentOperationStatus     `json:"status"`
	Amount                  models.Money               `json:"amount"`
	Provider                string                     `json:"provider"`
	ProviderTransactionID   string                     `json:"provider_transaction_id"`
	ExternalTransactionID   string                     `json:"external_transaction_id"`
	ErrorCode               string                     `json:"error_code,omitempty"`
	ErrorMessage            string                     `json:"error_message,omitempty"`
	Metadata                map[string]interface{}     `json:"metadata,omitempty"`
	Timestamps              models.Timestamps          `json:"timestamps"`
	Version                 models.Version             `json:"version"`

	events []*events.Event
}

// NewPaymentOperation creates a new payment operation
func NewPaymentOperation(
	paymentID models.ID,
	operationType PaymentOperationType,
	amount models.Money,
	provider string,
) *PaymentOperation {
	operation := &PaymentOperation{
		ID:         models.GenerateUUID(),
		PaymentID:  paymentID,
		Type:       operationType,
		Status:     PaymentOperationStatusPending,
		Amount:     amount,
		Provider:   provider,
		Metadata:   make(map[string]interface{}),
		Timestamps: models.NewTimestamps(),
		Version:    models.NewVersion(),
	}

	event := events.NewEvent(operation.ID, events.PaymentOperationCreatedEvent, PaymentOperationCreatedData{
		OperationID: operation.ID,
		PaymentID:   operation.PaymentID,
		Type:        operation.Type,
		Amount:      operation.Amount,
		Provider:    operation.Provider,
	})

	operation.recordEvent(event)
	return operation
}

// Process marks the operation as processing
func (po *PaymentOperation) Process() {
	po.Status = PaymentOperationStatusProcessing
	po.Timestamps = po.Timestamps.Update()
	po.Version = po.Version.Update()

	event := events.NewEvent(po.ID, events.PaymentOperationProcessingEvent, PaymentOperationProcessingData{
		OperationID: po.ID,
		PaymentID:   po.PaymentID,
	})

	po.recordEvent(event)
}

// Complete marks the operation as completed
func (po *PaymentOperation) Complete(providerTransactionID, externalTransactionID string) {
	po.Status = PaymentOperationStatusCompleted
	po.ProviderTransactionID = providerTransactionID
	po.ExternalTransactionID = externalTransactionID
	po.Timestamps = po.Timestamps.Update()
	po.Version = po.Version.Update()

	event := events.NewEvent(po.ID, events.PaymentOperationCompletedEvent, PaymentOperationCompletedData{
		OperationID:             po.ID,
		PaymentID:               po.PaymentID,
		Type:                    po.Type,
		Amount:                  po.Amount,
		ProviderTransactionID:   po.ProviderTransactionID,
		ExternalTransactionID:   po.ExternalTransactionID,
		CompletedAt:             time.Now(),
	})

	po.recordEvent(event)
}

// Fail marks the operation as failed
func (po *PaymentOperation) Fail(errorCode, errorMessage string) {
	po.Status = PaymentOperationStatusFailed
	po.ErrorCode = errorCode
	po.ErrorMessage = errorMessage
	po.Timestamps = po.Timestamps.Update()
	po.Version = po.Version.Update()

	event := events.NewEvent(po.ID, events.PaymentOperationFailedEvent, PaymentOperationFailedData{
		OperationID:  po.ID,
		PaymentID:    po.PaymentID,
		Type:         po.Type,
		Amount:       po.Amount,
		ErrorCode:    po.ErrorCode,
		ErrorMessage: po.ErrorMessage,
		FailedAt:     time.Now(),
	})

	po.recordEvent(event)
}

// Events returns domain events
func (po *PaymentOperation) Events() []*events.Event {
	return po.events
}

// ClearEvents clears domain events
func (po *PaymentOperation) ClearEvents() {
	po.events = make([]*events.Event, 0)
}

// recordEvent records a domain event
func (po *PaymentOperation) recordEvent(event *events.Event) {
	po.events = append(po.events, event)
}

// Event Data Structures
type PaymentOperationCreatedData struct {
	OperationID models.ID                `json:"operation_id"`
	PaymentID   models.ID                `json:"payment_id"`
	Type        PaymentOperationType     `json:"type"`
	Amount      models.Money             `json:"amount"`
	Provider    string                   `json:"provider"`
}

type PaymentOperationProcessingData struct {
	OperationID models.ID `json:"operation_id"`
	PaymentID   models.ID `json:"payment_id"`
}

type PaymentOperationCompletedData struct {
	OperationID             models.ID                `json:"operation_id"`
	PaymentID               models.ID                `json:"payment_id"`
	Type                    PaymentOperationType     `json:"type"`
	Amount                  models.Money             `json:"amount"`
	ProviderTransactionID   string                   `json:"provider_transaction_id"`
	ExternalTransactionID   string                   `json:"external_transaction_id"`
	CompletedAt             time.Time                `json:"completed_at"`
}

type PaymentOperationFailedData struct {
	OperationID  models.ID                `json:"operation_id"`
	PaymentID    models.ID                `json:"payment_id"`
	Type         PaymentOperationType     `json:"type"`
	Amount       models.Money             `json:"amount"`
	ErrorCode    string                   `json:"error_code"`
	ErrorMessage string                   `json:"error_message"`
	FailedAt     time.Time                `json:"failed_at"`
}