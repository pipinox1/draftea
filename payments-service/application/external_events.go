package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// ExternalWebhookPayload represents the generic webhook payload from external providers
type ExternalWebhookPayload struct {
	Provider         string                 `json:"provider"`
	EventType        string                 `json:"event_type"`
	TransactionID    string                 `json:"transaction_id"`
	ExternalID       string                 `json:"external_id"`
	PaymentReference string                 `json:"payment_reference"`
	Amount           int64                  `json:"amount"`
	Currency         string                 `json:"currency"`
	Status           string                 `json:"status"`
	ErrorCode        string                 `json:"error_code,omitempty"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
	Signature        string                 `json:"signature,omitempty"`
}

// HandleExternalWebhooksCommand represents the command to handle external webhooks
type HandleExternalWebhooksCommand struct {
	Provider  string `json:"provider"`
	Payload   []byte `json:"payload"`
	Signature string `json:"signature,omitempty"`
}

// HandleExternalWebhooks use case handles webhooks from external payment providers
type HandleExternalWebhooks struct {
	eventPublisher events.Publisher
}

// NewHandleExternalWebhooks creates a new HandleExternalWebhooks use case
func NewHandleExternalWebhooks(
	eventPublisher events.Publisher,
) *HandleExternalWebhooks {
	return &HandleExternalWebhooks{
		eventPublisher: eventPublisher,
	}
}

// Execute handles external webhook and publishes corresponding events
func (uc *HandleExternalWebhooks) Execute(ctx context.Context, cmd *HandleExternalWebhooksCommand) error {
	// Validate command
	if err := uc.validateCommand(cmd); err != nil {
		return errors.Wrap(err, "invalid command")
	}

	// Parse webhook payload based on provider
	webhookData, err := uc.parseWebhookPayload(cmd.Provider, cmd.Payload)
	if err != nil {
		return errors.Wrap(err, "failed to parse webhook payload")
	}

	// Verify webhook signature if provided (provider-specific verification would go here)
	if err := uc.verifyWebhookSignature(cmd.Provider, cmd.Payload, cmd.Signature); err != nil {
		return errors.Wrap(err, "webhook signature verification failed")
	}

	// Create external provider update event
	paymentID, err := models.NewID(webhookData.PaymentReference)
	if err != nil {
		return errors.Wrap(err, "invalid payment reference")
	}

	updateEvent := events.NewEvent(
		paymentID,
		events.ExternalProviderUpdateEvent,
		ExternalProviderUpdateData{
			Provider:         webhookData.Provider,
			EventType:        webhookData.EventType,
			TransactionID:    webhookData.TransactionID,
			ExternalID:       webhookData.ExternalID,
			PaymentReference: webhookData.PaymentReference,
			Amount:           models.NewMoney(webhookData.Amount, webhookData.Currency),
			Status:           webhookData.Status,
			ErrorCode:        webhookData.ErrorCode,
			ErrorMessage:     webhookData.ErrorMessage,
			Metadata:         webhookData.Metadata,
			Timestamp:        webhookData.Timestamp,
		},
	)

	// Publish the event for processing
	if err := uc.eventPublisher.Publish(ctx, updateEvent); err != nil {
		return errors.Wrap(err, "failed to publish external provider update event")
	}

	return nil
}

// parseWebhookPayload parses webhook payload based on provider
func (uc *HandleExternalWebhooks) parseWebhookPayload(provider string, payload []byte) (*ExternalWebhookPayload, error) {
	var webhookData ExternalWebhookPayload

	switch provider {
	case "stripe":
		// Parse Stripe webhook format
		if err := uc.parseStripeWebhook(payload, &webhookData); err != nil {
			return nil, errors.Wrap(err, "failed to parse Stripe webhook")
		}

	case "external_gateway":
		// Parse generic external gateway webhook format
		if err := json.Unmarshal(payload, &webhookData); err != nil {
			return nil, errors.Wrap(err, "failed to parse external gateway webhook")
		}

	default:
		return nil, errors.New("unsupported webhook provider")
	}

	webhookData.Provider = provider
	return &webhookData, nil
}

// parseStripeWebhook parses Stripe-specific webhook format
func (uc *HandleExternalWebhooks) parseStripeWebhook(payload []byte, webhookData *ExternalWebhookPayload) error {
	// This is a simplified Stripe webhook parser
	// In production, you'd use the Stripe SDK to properly parse and verify webhooks
	var stripeEvent map[string]interface{}
	if err := json.Unmarshal(payload, &stripeEvent); err != nil {
		return err
	}

	webhookData.EventType = stripeEvent["type"].(string)
	webhookData.Timestamp = time.Now()

	// Extract payment intent data (simplified)
	if data, ok := stripeEvent["data"].(map[string]interface{}); ok {
		if object, ok := data["object"].(map[string]interface{}); ok {
			if id, ok := object["id"].(string); ok {
				webhookData.TransactionID = id
			}
			if amount, ok := object["amount"].(float64); ok {
				webhookData.Amount = int64(amount)
			}
			if currency, ok := object["currency"].(string); ok {
				webhookData.Currency = currency
			}
			if status, ok := object["status"].(string); ok {
				webhookData.Status = status
			}
			if metadata, ok := object["metadata"].(map[string]interface{}); ok {
				if paymentRef, ok := metadata["payment_reference"].(string); ok {
					webhookData.PaymentReference = paymentRef
				}
			}
		}
	}

	return nil
}

// verifyWebhookSignature verifies webhook signature based on provider
func (uc *HandleExternalWebhooks) verifyWebhookSignature(provider string, payload []byte, signature string) error {
	// In production, implement proper signature verification for each provider
	// For now, we'll skip verification if no signature is provided
	if signature == "" {
		return nil
	}

	switch provider {
	case "stripe":
		// Implement Stripe signature verification
		// stripe.VerifyWebhookSignature(payload, signature, webhookSecret)
		return nil

	case "external_gateway":
		// Implement external gateway signature verification
		return nil

	default:
		return errors.New("unsupported provider for signature verification")
	}
}

// validateCommand validates the handle external webhooks command
func (uc *HandleExternalWebhooks) validateCommand(cmd *HandleExternalWebhooksCommand) error {
	if cmd.Provider == "" {
		return errors.New("provider is required")
	}

	if len(cmd.Payload) == 0 {
		return errors.New("payload is required")
	}

	return nil
}

// ExternalProviderUpdateData represents data for external provider update event
type ExternalProviderUpdateData struct {
	Provider         string                 `json:"provider"`
	EventType        string                 `json:"event_type"`
	TransactionID    string                 `json:"transaction_id"`
	ExternalID       string                 `json:"external_id"`
	PaymentReference string                 `json:"payment_reference"`
	Amount           models.Money           `json:"amount"`
	Status           string                 `json:"status"`
	ErrorCode        string                 `json:"error_code,omitempty"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
}
