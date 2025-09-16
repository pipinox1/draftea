package events

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/draftea/payment-system/shared/models"
)

var (
	ErrInvalidTopic    = errors.New("invalid topic")
	ErrInvalidPayload  = errors.New("invalid payload")
	ErrInvalidReceiver = errors.New("receiver should be a pointer")
	ErrInvalidHandleID = errors.New("invalid handle ID")
)

// Topic represents an event topic with pattern matching support
type Topic string

func NewTopic(topic string) (Topic, error) {
	if topic == "" {
		return "", ErrInvalidTopic
	}
	return Topic(topic), nil
}

func (t Topic) Matches(pattern Topic) bool {
	topicStr := t.String()
	patternStr := pattern.String()

	if strings.HasPrefix(patternStr, "#") && strings.HasSuffix(patternStr, "#") {
		return strings.Contains(
			topicStr,
			strings.TrimSuffix(strings.TrimPrefix(patternStr, "#"), "#"),
		)
	}

	if strings.HasPrefix(patternStr, "#") {
		return strings.HasSuffix(
			topicStr,
			strings.TrimPrefix(patternStr, "#"),
		)
	}

	if strings.HasSuffix(patternStr, "#") {
		return strings.HasPrefix(
			topicStr,
			strings.TrimSuffix(patternStr, "#"),
		)
	}

	patternParts := strings.Split(patternStr, ".")
	topicParts := strings.Split(topicStr, ".")

	return matchPattern(patternParts, topicParts)
}

func (t Topic) String() string {
	return string(t)
}

func matchPattern(patternParts, topicParts []string) bool {
	if len(patternParts) == 1 && patternParts[0] == "#" {
		return true
	}

	if len(patternParts) != len(topicParts) {
		return false
	}

	if len(patternParts) == 0 {
		return true
	}

	if patternParts[0] == "*" || patternParts[0] == topicParts[0] {
		return matchPattern(patternParts[1:], topicParts[1:])
	}

	return false
}

// Metadata represents event metadata
type Metadata map[string]string

func (m Metadata) Get(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

func (m Metadata) Set(key string, value string) {
	if m == nil {
		m = make(Metadata)
	}
	m[key] = value
}

func (m Metadata) Delete(key string) {
	delete(m, key)
}

func (m Metadata) Has(key string) bool {
	_, ok := m[key]
	return ok
}

func (m Metadata) Merge(metadata Metadata) Metadata {
	if m == nil {
		m = make(Metadata)
	}
	for k, v := range metadata {
		m[k] = v
	}
	return m
}

func (m Metadata) Matches(o Metadata) bool {
	for k, v := range o {
		if m[k] != v {
			return false
		}
	}
	return true
}

func (m Metadata) Clone() Metadata {
	clone := Metadata{}
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// Event represents a domain event
type Event struct {
	ID            models.ID   `json:"id"`
	AggregateID   models.ID   `json:"aggregate_id"`
	Topic         Topic       `json:"topic"`
	EventType     string      `json:"event_type"` // Kept for backward compatibility
	Version       string      `json:"version"`
	Data          interface{} `json:"data"`
	Metadata      Metadata    `json:"metadata"`
	Timestamp     time.Time   `json:"timestamp"`
	CorrelationID models.ID   `json:"correlation_id"`
}

// Publisher publishes events
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
	topic, _ := NewTopic(eventType) // We'll ignore error since we trust eventType constants
	return &Event{
		ID:          models.GenerateUUID(),
		AggregateID: aggregateID,
		Topic:       topic,
		EventType:   eventType,
		Version:     "1.0",
		Data:        data,
		Metadata:    make(Metadata),
		Timestamp:   time.Now(),
	}
}

// NewEventWithTopic creates a new domain event with explicit topic
func NewEventWithTopic(aggregateID models.ID, topic Topic, data interface{}) *Event {
	return &Event{
		ID:          models.GenerateUUID(),
		AggregateID: aggregateID,
		Topic:       topic,
		EventType:   topic.String(), // Set EventType from topic for backward compatibility
		Version:     "1.0",
		Data:        data,
		Metadata:    make(Metadata),
		Timestamp:   time.Now(),
	}
}

// WithCorrelationID sets correlation ID
func (e *Event) WithCorrelationID(correlationID models.ID) *Event {
	e.CorrelationID = correlationID
	return e
}

// WithMetadata adds metadata
func (e *Event) WithMetadata(key string, value string) *Event {
	if e.Metadata == nil {
		e.Metadata = make(Metadata)
	}
	e.Metadata.Set(key, value)
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

// MarshalPayload marshals the event payload
func (e *Event) MarshalPayload() (json.RawMessage, error) {
	if b, ok := e.Data.([]byte); ok {
		return b, nil
	}

	if b, ok := e.Data.(json.RawMessage); ok {
		return b, nil
	}

	return json.Marshal(e.Data)
}

// UnmarshalPayload unmarshals the event payload into the given interface
func (e *Event) UnmarshalPayload(v interface{}) error {
	vValue := reflect.ValueOf(v)
	if vValue.Kind() != reflect.Ptr {
		return ErrInvalidReceiver
	}

	vValue = vValue.Elem()
	payloadValue := reflect.ValueOf(e.Data)
	if vValue.Type() == payloadValue.Type() {
		vValue.Set(payloadValue)
		return nil
	}

	if b, ok := e.Data.([]byte); ok {
		return json.Unmarshal(b, v)
	}

	if b, ok := e.Data.(json.RawMessage); ok {
		return json.Unmarshal([]byte(b), v)
	}

	raw, err := e.MarshalPayload()
	if err != nil {
		return err
	}

	return json.Unmarshal(raw, v)
}

// Matches checks if the event matches the given topic pattern and metadata
func (e *Event) Matches(topicPattern Topic, metadata Metadata) bool {
	return e.Topic.Matches(topicPattern) && e.Metadata.Matches(metadata)
}

// Clone creates a copy of the event
func (e *Event) Clone() *Event {
	return &Event{
		ID:            e.ID,
		AggregateID:   e.AggregateID,
		Topic:         e.Topic,
		EventType:     e.EventType,
		Version:       e.Version,
		Data:          e.Data,
		Metadata:      e.Metadata.Clone(),
		Timestamp:     e.Timestamp,
		CorrelationID: e.CorrelationID,
	}
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
