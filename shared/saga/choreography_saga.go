package saga

import (
	"context"
	"fmt"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
)

// ChoreographySaga implements saga pattern using choreography (event-driven)
// Each service reacts to events and publishes new events without a central coordinator

// PaymentProcessingChoreography handles payment processing using choreography pattern
type PaymentProcessingChoreography struct {
	eventPublisher events.Publisher
	eventStore     events.EventStore
}

// NewPaymentProcessingChoreography creates a new choreography-based payment processing saga
func NewPaymentProcessingChoreography(eventPublisher events.Publisher, eventStore events.EventStore) *PaymentProcessingChoreography {
	return &PaymentProcessingChoreography{
		eventPublisher: eventPublisher,
		eventStore:     eventStore,
	}
}

// Payment Service Event Handlers (Choreography)

// PaymentInitiatedHandler handles PaymentInitiated events
type PaymentInitiatedHandler struct {
	eventPublisher events.Publisher
}

func NewPaymentInitiatedHandler(eventPublisher events.Publisher) *PaymentInitiatedHandler {
	return &PaymentInitiatedHandler{eventPublisher: eventPublisher}
}

func (h *PaymentInitiatedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentCreatedEvent {
		return nil
	}

	// Extract payment data
	data := event.Data.(map[string]interface{})
	paymentMethod := data["payment_method"].(map[string]interface{})

	// If payment method is wallet, request wallet debit
	if paymentMethod["type"] == "wallet" {
		walletDebitEvent := events.NewEvent(
			models.ID(paymentMethod["wallet_id"].(string)),
			events.WalletDebitRequestedEvent,
			map[string]interface{}{
				"wallet_id":  paymentMethod["wallet_id"],
				"payment_id": data["payment_id"],
				"user_id":    data["user_id"],
				"amount":     data["amount"],
				"currency":   data["currency"],
				"reference":  fmt.Sprintf("Payment %s", data["payment_id"]),
			},
		).WithCorrelationID(event.AggregateID)

		return h.eventPublisher.Publish(ctx, walletDebitEvent)
	}

	// For non-wallet payments, go directly to gateway processing
	gatewayEvent := events.NewEvent(
		event.AggregateID,
		"gateway.processing.requested",
		map[string]interface{}{
			"payment_id": data["payment_id"],
			"amount":     data["amount"],
			"currency":   data["currency"],
			"gateway":    "stripe", // Default gateway
		},
	).WithCorrelationID(event.AggregateID)

	return h.eventPublisher.Publish(ctx, gatewayEvent)
}

// WalletDebitedHandler handles WalletDebited events to continue payment flow
type WalletDebitedHandler struct {
	eventPublisher events.Publisher
}

func NewWalletDebitedHandler(eventPublisher events.Publisher) *WalletDebitedHandler {
	return &WalletDebitedHandler{eventPublisher: eventPublisher}
}

func (h *WalletDebitedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != events.WalletDebitedEvent {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})
	paymentID := data["payment_id"].(string)

	// Wallet debited successfully, now process with gateway
	gatewayEvent := events.NewEvent(
		models.ID(paymentID),
		"gateway.processing.requested",
		map[string]interface{}{
			"payment_id": paymentID,
			"amount":     data["amount"],
			"currency":   data["currency"],
			"gateway":    "stripe",
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, gatewayEvent)
}

// InsufficientFundsHandler handles InsufficientFunds events to fail payment
type InsufficientFundsHandler struct {
	eventPublisher events.Publisher
}

func NewInsufficientFundsHandler(eventPublisher events.Publisher) *InsufficientFundsHandler {
	return &InsufficientFundsHandler{eventPublisher: eventPublisher}
}

func (h *InsufficientFundsHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != events.InsufficientFundsEvent {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})
	paymentID := data["payment_id"].(string)

	// Fail the payment due to insufficient funds
	paymentFailEvent := events.NewEvent(
		models.ID(paymentID),
		"payment.failure.requested",
		map[string]interface{}{
			"payment_id": paymentID,
			"reason":     "Insufficient funds in wallet",
			"error_code": "INSUFFICIENT_FUNDS",
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, paymentFailEvent)
}

// GatewayProcessingCompletedHandler handles successful gateway processing
type GatewayProcessingCompletedHandler struct {
	eventPublisher events.Publisher
}

func NewGatewayProcessingCompletedHandler(eventPublisher events.Publisher) *GatewayProcessingCompletedHandler {
	return &GatewayProcessingCompletedHandler{eventPublisher: eventPublisher}
}

func (h *GatewayProcessingCompletedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != "gateway.processing.completed" {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})
	paymentID := data["payment_id"].(string)
	gatewayTransactionID := data["gateway_transaction_id"].(string)

	// Complete the payment
	paymentCompleteEvent := events.NewEvent(
		models.ID(paymentID),
		"payment.completion.requested",
		map[string]interface{}{
			"payment_id":             paymentID,
			"gateway_transaction_id": gatewayTransactionID,
			"transaction_id":         models.GenerateUUID().String(),
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, paymentCompleteEvent)
}

// GatewayProcessingFailedHandler handles failed gateway processing
type GatewayProcessingFailedHandler struct {
	eventPublisher events.Publisher
}

func NewGatewayProcessingFailedHandler(eventPublisher events.Publisher) *GatewayProcessingFailedHandler {
	return &GatewayProcessingFailedHandler{eventPublisher: eventPublisher}
}

func (h *GatewayProcessingFailedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != "gateway.processing.failed" {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})
	paymentID := data["payment_id"].(string)
	gatewayError := data["error"].(string)

	// Check if we need to compensate wallet debit
	if walletID, exists := data["wallet_id"]; exists {
		// Compensate wallet debit by crediting back
		walletCreditEvent := events.NewEvent(
			models.ID(walletID.(string)),
			"wallet.credit.requested",
			map[string]interface{}{
				"wallet_id":  walletID,
				"payment_id": paymentID,
				"amount":     data["amount"],
				"currency":   data["currency"],
				"reference":  fmt.Sprintf("Payment %s compensation", paymentID),
			},
		).WithCorrelationID(event.CorrelationID)

		if err := h.eventPublisher.Publish(ctx, walletCreditEvent); err != nil {
			return fmt.Errorf("failed to publish wallet compensation event: %w", err)
		}
	}

	// Fail the payment
	paymentFailEvent := events.NewEvent(
		models.ID(paymentID),
		"payment.failure.requested",
		map[string]interface{}{
			"payment_id": paymentID,
			"reason":     fmt.Sprintf("Gateway processing failed: %s", gatewayError),
			"error_code": "GATEWAY_FAILED",
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, paymentFailEvent)
}

// Payment Request/Completion Event Handlers

// PaymentCompletionRequestedHandler handles payment completion requests
type PaymentCompletionRequestedHandler struct {
	eventPublisher events.Publisher
}

func NewPaymentCompletionRequestedHandler(eventPublisher events.Publisher) *PaymentCompletionRequestedHandler {
	return &PaymentCompletionRequestedHandler{eventPublisher: eventPublisher}
}

func (h *PaymentCompletionRequestedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != "payment.completion.requested" {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})

	// Publish payment completed event
	paymentCompletedEvent := events.NewEvent(
		event.AggregateID,
		events.PaymentCompletedEvent,
		map[string]interface{}{
			"payment_id":             data["payment_id"],
			"transaction_id":         data["transaction_id"],
			"gateway_transaction_id": data["gateway_transaction_id"],
			"completed_at":           fmt.Sprintf("%d", event.Timestamp.Unix()),
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, paymentCompletedEvent)
}

// PaymentFailureRequestedHandler handles payment failure requests
type PaymentFailureRequestedHandler struct {
	eventPublisher events.Publisher
}

func NewPaymentFailureRequestedHandler(eventPublisher events.Publisher) *PaymentFailureRequestedHandler {
	return &PaymentFailureRequestedHandler{eventPublisher: eventPublisher}
}

func (h *PaymentFailureRequestedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != "payment.failure.requested" {
		return nil
	}

	// Extract data
	data := event.Data.(map[string]interface{})

	// Publish payment failed event
	paymentFailedEvent := events.NewEvent(
		event.AggregateID,
		events.PaymentFailedEvent,
		map[string]interface{}{
			"payment_id": data["payment_id"],
			"reason":     data["reason"],
			"error_code": data["error_code"],
			"failed_at":  fmt.Sprintf("%d", event.Timestamp.Unix()),
		},
	).WithCorrelationID(event.CorrelationID)

	return h.eventPublisher.Publish(ctx, paymentFailedEvent)
}

// Wallet Service Choreography Handlers

// WalletDebitRequestedHandler handles wallet debit requests
type WalletDebitRequestedHandler struct {
	eventPublisher events.Publisher
}

func NewWalletDebitRequestedHandler(eventPublisher events.Publisher) *WalletDebitRequestedHandler {
	return &WalletDebitRequestedHandler{eventPublisher: eventPublisher}
}

func (h *WalletDebitRequestedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != events.WalletDebitRequestedEvent {
		return nil
	}

	// This handler would trigger the wallet service to process the debit
	// The wallet service would then publish either WalletDebitedEvent or InsufficientFundsEvent
	// For now, we'll just log that the request was received
	fmt.Printf("Wallet debit requested: %+v\n", event.Data)
	return nil
}

// WalletCreditRequestedHandler handles wallet credit requests (compensations)
type WalletCreditRequestedHandler struct {
	eventPublisher events.Publisher
}

func NewWalletCreditRequestedHandler(eventPublisher events.Publisher) *WalletCreditRequestedHandler {
	return &WalletCreditRequestedHandler{eventPublisher: eventPublisher}
}

func (h *WalletCreditRequestedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.EventType != "wallet.credit.requested" {
		return nil
	}

	// This handler would trigger the wallet service to process the credit
	// The wallet service would then publish WalletCreditedEvent
	fmt.Printf("Wallet credit requested: %+v\n", event.Data)
	return nil
}

// ChoreographyEventRouter routes events to appropriate handlers
type ChoreographyEventRouter struct {
	handlers map[string][]events.EventHandler
}

// NewChoreographyEventRouter creates a new event router for choreography
func NewChoreographyEventRouter() *ChoreographyEventRouter {
	return &ChoreographyEventRouter{
		handlers: make(map[string][]events.EventHandler),
	}
}

// RegisterHandler registers an event handler for a specific event type
func (r *ChoreographyEventRouter) RegisterHandler(eventType string, handler events.EventHandler) {
	r.handlers[eventType] = append(r.handlers[eventType], handler)
}

// Route routes an event to all registered handlers
func (r *ChoreographyEventRouter) Route(ctx context.Context, event *events.Event) error {
	handlers, exists := r.handlers[event.EventType]
	if !exists {
		fmt.Printf("No handlers registered for event type: %s\n", event.EventType)
		return nil
	}

	for _, handler := range handlers {
		if err := handler.Handle(ctx, event); err != nil {
			fmt.Printf("Handler failed for event %s: %v\n", event.EventType, err)
			// In a production system, you might want to publish a failure event
			// or implement retry logic
		}
	}

	return nil
}
