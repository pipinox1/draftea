package saga

// This file contains shared saga interfaces and types for choreography pattern.
// Orchestration-based saga implementation has been removed in favor of choreography.

// SagaStatus represents the current status of a saga (used for tracking only)
type SagaStatus string

const (
	SagaStatusStarted    SagaStatus = "started"
	SagaStatusInProgress SagaStatus = "in_progress"
	SagaStatusCompleted  SagaStatus = "completed"
	SagaStatusFailed     SagaStatus = "failed"
)

// Note: In choreography pattern, there is no central orchestrator.
// Each service listens for events and publishes new events as part of the business flow.
// Compensation is handled by individual services when they receive failure events.