package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// PostgresEventStore implements EventStore using PostgreSQL
type PostgresEventStore struct {
	db *sqlx.DB
}

// NewPostgresEventStore creates a new PostgresEventStore
func NewPostgresEventStore(db *sqlx.DB) *PostgresEventStore {
	return &PostgresEventStore{db: db}
}

// postgresEvent represents event in database
type postgresEvent struct {
	ID           string                 `db:"id"`
	AggregateID  string                 `db:"aggregate_id"`
	EventType    string                 `db:"event_type"`
	Version      string                 `db:"version"`
	Data         []byte                 `db:"data"`
	Metadata     []byte                 `db:"metadata"`
	Timestamp    time.Time              `db:"timestamp"`
	CorrelationID string                `db:"correlation_id"`
	StreamVersion int                   `db:"stream_version"`
}

// SaveEvents saves events to the event store
func (es *PostgresEventStore) SaveEvents(ctx context.Context, aggregateID models.ID, events []*events.Event, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := es.db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Check current version
	var currentVersion int
	err = tx.GetContext(ctx, &currentVersion,
		"SELECT COALESCE(MAX(stream_version), 0) FROM event_stream WHERE aggregate_id = $1",
		aggregateID.String())
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "failed to get current version")
	}

	if currentVersion != expectedVersion {
		return errors.Errorf("concurrency conflict: expected version %d, got %d", expectedVersion, currentVersion)
	}

	// Insert events
	for i, event := range events {
		pgEvent, err := es.toPostgres(event, currentVersion+i+1)
		if err != nil {
			return errors.Wrap(err, "failed to convert event")
		}

		query := `
			INSERT INTO event_stream (
				id, aggregate_id, event_type, version, data, metadata,
				timestamp, correlation_id, stream_version
			) VALUES (
				:id, :aggregate_id, :event_type, :version, :data, :metadata,
				:timestamp, :correlation_id, :stream_version
			)`

		_, err = tx.NamedExecContext(ctx, query, pgEvent)
		if err != nil {
			return errors.Wrap(err, "failed to insert event")
		}
	}

	return tx.Commit()
}

// GetEvents retrieves all events for an aggregate
func (es *PostgresEventStore) GetEvents(ctx context.Context, aggregateID models.ID) ([]*events.Event, error) {
	query := `
		SELECT id, aggregate_id, event_type, version, data, metadata,
			   timestamp, correlation_id, stream_version
		FROM event_stream
		WHERE aggregate_id = $1
		ORDER BY stream_version ASC`

	var pgEvents []postgresEvent
	err := es.db.SelectContext(ctx, &pgEvents, query, aggregateID.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get events")
	}

	events := make([]*events.Event, len(pgEvents))
	for i, pgEvent := range pgEvents {
		event, err := es.toDomain(&pgEvent)
		if err != nil {
			return nil, err
		}
		events[i] = event
	}

	return events, nil
}

// GetEventsByType retrieves events by type with pagination
func (es *PostgresEventStore) GetEventsByType(ctx context.Context, eventType string, offset, limit int) ([]*events.Event, error) {
	query := `
		SELECT id, aggregate_id, event_type, version, data, metadata,
			   timestamp, correlation_id, stream_version
		FROM event_stream
		WHERE event_type = $1
		ORDER BY timestamp ASC
		LIMIT $2 OFFSET $3`

	var pgEvents []postgresEvent
	err := es.db.SelectContext(ctx, &pgEvents, query, eventType, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get events by type")
	}

	events := make([]*events.Event, len(pgEvents))
	for i, pgEvent := range pgEvents {
		event, err := es.toDomain(&pgEvent)
		if err != nil {
			return nil, err
		}
		events[i] = event
	}

	return events, nil
}

// toPostgres converts domain event to postgres model
func (es *PostgresEventStore) toPostgres(event *events.Event, streamVersion int) (*postgresEvent, error) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal event data")
	}

	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal event metadata")
	}

	correlationID := ""
	if event.CorrelationID != "" {
		correlationID = event.CorrelationID.String()
	}

	return &postgresEvent{
		ID:            event.ID.String(),
		AggregateID:   event.AggregateID.String(),
		EventType:     event.EventType,
		Version:       event.Version,
		Data:          data,
		Metadata:      metadata,
		Timestamp:     event.Timestamp,
		CorrelationID: correlationID,
		StreamVersion: streamVersion,
	}, nil
}

// toDomain converts postgres model to domain event
func (es *PostgresEventStore) toDomain(pgEvent *postgresEvent) (*events.Event, error) {
	id, err := models.NewID(pgEvent.ID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid event ID")
	}

	aggregateID, err := models.NewID(pgEvent.AggregateID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid aggregate ID")
	}

	var data interface{}
	if err := json.Unmarshal(pgEvent.Data, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal event data")
	}

	var rawMetadata map[string]interface{}
	if err := json.Unmarshal(pgEvent.Metadata, &rawMetadata); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal event metadata")
	}

	// Convert to events.Metadata (map[string]string)
	metadata := make(events.Metadata)
	for k, v := range rawMetadata {
		if str, ok := v.(string); ok {
			metadata.Set(k, str)
		} else {
			metadata.Set(k, fmt.Sprintf("%v", v))
		}
	}

	var correlationID models.ID
	if pgEvent.CorrelationID != "" {
		correlationID, err = models.NewID(pgEvent.CorrelationID)
		if err != nil {
			return nil, errors.Wrap(err, "invalid correlation ID")
		}
	}

	// Create topic from event type for backward compatibility
	topic, _ := events.NewTopic(pgEvent.EventType)

	return &events.Event{
		ID:            id,
		AggregateID:   aggregateID,
		Topic:         topic,
		EventType:     pgEvent.EventType,
		Version:       pgEvent.Version,
		Data:          data,
		Metadata:      metadata,
		Timestamp:     pgEvent.Timestamp,
		CorrelationID: correlationID,
	}, nil
}