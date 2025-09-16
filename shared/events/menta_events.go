package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/draftea/payment-system/shared/models"
	"github.com/google/uuid"
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

// MentaEvent represents a domain event using Menta's structure
type MentaEvent struct {
	ID        string      `json:"id"`
	Topic     Topic       `json:"topic"`
	Metadata  Metadata    `json:"metadata"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewMentaEvent(
	topic Topic,
	payload interface{},
	metadata ...Metadata,
) (*MentaEvent, error) {
	if topic == "" {
		return nil, ErrInvalidTopic
	}

	m := Metadata{}
	for _, md := range metadata {
		m = m.Merge(md)
	}

	if payload == nil {
		return nil, ErrInvalidPayload
	}

	return &MentaEvent{
		ID:        uuid.New().String(),
		Topic:     topic,
		Metadata:  m,
		Payload:   payload,
		Timestamp: time.Now(),
	}, nil
}

func (e *MentaEvent) MarshalPayload() (json.RawMessage, error) {
	if b, ok := e.Payload.([]byte); ok {
		return b, nil
	}

	if b, ok := e.Payload.(json.RawMessage); ok {
		return b, nil
	}

	return json.Marshal(e.Payload)
}

func (e *MentaEvent) UnmarshalPayload(v interface{}) error {
	vValue := reflect.ValueOf(v)
	if vValue.Kind() != reflect.Ptr {
		return ErrInvalidReceiver
	}

	vValue = vValue.Elem()
	payloadValue := reflect.ValueOf(e.Payload)
	if vValue.Type() == payloadValue.Type() {
		vValue.Set(payloadValue)
		return nil
	}

	if b, ok := e.Payload.([]byte); ok {
		return json.Unmarshal(b, v)
	}

	if b, ok := e.Payload.(json.RawMessage); ok {
		return json.Unmarshal([]byte(b), v)
	}

	raw, err := e.MarshalPayload()
	if err != nil {
		return err
	}

	return json.Unmarshal(raw, v)
}

func (e *MentaEvent) Matches(topic Topic, metadata Metadata) bool {
	return e.Topic.Matches(topic) && e.Metadata.Matches(metadata)
}

func (e *MentaEvent) Clone() *MentaEvent {
	return &MentaEvent{
		ID:        e.ID,
		Topic:     e.Topic,
		Metadata:  e.Metadata.Clone(),
		Payload:   e.Payload,
		Timestamp: e.Timestamp,
	}
}

// EventDTO represents event data transfer object
type EventDTO struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Metadata  Metadata        `json:"metadata"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

func (e *MentaEvent) MarshalJSON() ([]byte, error) {
	payload, err := e.MarshalPayload()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&EventDTO{
		ID:        e.ID,
		Topic:     string(e.Topic),
		Metadata:  e.Metadata,
		Payload:   payload,
		Timestamp: e.Timestamp,
	})
}

func (e *MentaEvent) UnmarshalJSON(b []byte) error {
	var dto *EventDTO
	if err := json.Unmarshal(b, &dto); err != nil {
		return err
	}

	e.ID = dto.ID

	topic, err := NewTopic(dto.Topic)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidTopic, err)
	}

	e.Topic = topic
	e.Metadata = dto.Metadata
	e.Payload = dto.Payload
	e.Timestamp = dto.Timestamp

	if e.Metadata == nil {
		e.Metadata = Metadata{}
	}

	return nil
}

// Convert our existing Event to MentaEvent
func ToMentaEvent(event *Event) (*MentaEvent, error) {
	topic, err := NewTopic(event.EventType)
	if err != nil {
		return nil, err
	}

	metadata := make(Metadata)
	for k, v := range event.Metadata {
		if str, ok := v.(string); ok {
			metadata.Set(k, str)
		} else {
			metadata.Set(k, fmt.Sprintf("%v", v))
		}
	}

	return &MentaEvent{
		ID:        event.ID.String(),
		Topic:     topic,
		Metadata:  metadata,
		Payload:   event.Data,
		Timestamp: event.Timestamp,
	}, nil
}

// Convert MentaEvent to our existing Event
func FromMentaEvent(mentaEvent *MentaEvent) (*Event, error) {
	eventMeta := make(map[string]interface{})
	for k, v := range mentaEvent.Metadata {
		eventMeta[k] = v
	}

	return &Event{
		ID:          models.ID(mentaEvent.ID),
		AggregateID: models.ID(""), // Will need to be set from metadata if needed
		EventType:   mentaEvent.Topic.String(),
		Version:     "1.0",
		Data:        mentaEvent.Payload,
		Metadata:    eventMeta,
		Timestamp:   mentaEvent.Timestamp,
	}, nil
}