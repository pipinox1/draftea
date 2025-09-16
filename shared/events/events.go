package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/draftea/payment-system/shared/models"
)

// Event represents a domain event
type Event struct {
	ID            models.ID              `json:"id"`
	AggregateID   models.ID              `json:"aggregate_id"`
	EventType     string                 `json:"event_type"`
	Version       string                 `json:"version"`
	Data          interface{}            `json:"data"`
	Metadata      map[string]interface{} `json:"metadata"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID models.ID              `json:"correlation_id"`
}

// Publisher publishes events to event store
type Publisher interface {
	Publish(ctx context.Context, events ...*Event) error
}

// Subscriber subscribes to events
type Subscriber interface {
	Subscribe(ctx context.Context, eventType string, handler EventHandler) error
}

// EventHandler handles domain events
type EventHandler interface {
	Handle(ctx context.Context, event *Event) error
}

// EventStore stores and retrieves events
type EventStore interface {
	SaveEvents(ctx context.Context, aggregateID models.ID, events []*Event, expectedVersion int) error
	GetEvents(ctx context.Context, aggregateID models.ID) ([]*Event, error)
	GetEventsByType(ctx context.Context, eventType string, offset, limit int) ([]*Event, error)
}

// NewEvent creates a new domain event
func NewEvent(aggregateID models.ID, eventType string, data interface{}) *Event {
	return &Event{
		ID:          models.GenerateUUID(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Version:     "1.0",
		Data:        data,
		Metadata:    make(map[string]interface{}),
		Timestamp:   time.Now(),
	}
}

// WithCorrelationID sets correlation ID
func (e *Event) WithCorrelationID(correlationID models.ID) *Event {
	e.CorrelationID = correlationID
	return e
}

// WithMetadata adds metadata
func (e *Event) WithMetadata(key string, value interface{}) *Event {
	e.Metadata[key] = value
	return e
}

// ToJSON converts event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON creates event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	err := json.Unmarshal(data, &event)
	return &event, err
}

// Event Types Constants
const (
	// Payment Events
	PaymentCreatedEvent = "payment.created"

	PaymentProcessingEvent                     = "payment.processing"
	PaymentCompletedEvent                      = "payment.completed"
	PaymentFailedEvent                         = "payment.failed"
	PaymentCancelledEvent                      = "payment.cancelled"
	PaymentRefundInitiatedEvent                = "payment.refund.initiated"
	PaymentRefundCompletedEvent                = "payment.refund.completed"
	PaymentRefundFailedEvent                   = "payment.refund.failed"
	PaymentInconsistentStateEvent              = "payment.inconsistent.state"
	PaymentInconsistentOperationStartedEvent   = "payment.inconsistent.operation.started"
	PaymentInconsistentOperationProcessedEvent = "payment.inconsistent.operation.processed"

	// Payment Operation Events
	PaymentOperationCreatedEvent    = "payment.operation.created"
	PaymentOperationProcessingEvent = "payment.operation.processing"
	PaymentOperationCompletedEvent  = "payment.operation.completed"
	PaymentOperationFailedEvent     = "payment.operation.failed"

	// External Provider Events
	ExternalProviderUpdateEvent = "external.provider.update"

	// Wallet Events
	WalletDebitRequestedEvent            = "wallet.debit.requested"
	WalletCreditRequestedEvent           = "wallet.credit.requested"
	WalletDebitedEvent                   = "wallet.debited"
	WalletCreditedEvent                  = "wallet.credited"
	WalletMovementCreatedEvent           = "wallet.movement.created"
	WalletMovementRevertedEvent          = "wallet.movement.reverted"
	WalletMovementCreationRequestedEvent = "wallet.movement.creation.requested"
	WalletMovementRevertRequestedEvent   = "wallet.movement.revert.requested"
	InsufficientFundsEvent               = "wallet.insufficient.funds"
	WalletFrozenEvent                    = "wallet.frozen"
	WalletUnfrozenEvent                  = "wallet.unfrozen"

	// Saga Events
	SagaStartedEvent     = "saga.started"
	SagaCompletedEvent   = "saga.completed"
	SagaFailedEvent      = "saga.failed"
	SagaCompensatedEvent = "saga.compensated"
)
