package saga

import (
	"context"
	"fmt"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
)

// SQSChoreographyEventRouter routes events to appropriate handlers using SQS/SNS
type SQSChoreographyEventRouter struct {
	handlers map[string][]SQSEventHandler
}

// SQSEventHandler interface for handling events
type SQSEventHandler interface {
	Handle(ctx context.Context, event *events.Event) error
}

// SQSEventHandlerFunc wraps function as handler
type SQSEventHandlerFunc struct {
	fn func(ctx context.Context, event *events.Event) error
}

func NewSQSEventHandlerFunc(fn func(ctx context.Context, event *events.Event) error) *SQSEventHandlerFunc {
	return &SQSEventHandlerFunc{fn: fn}
}

func (h *SQSEventHandlerFunc) Handle(ctx context.Context, event *events.Event) error {
	return h.fn(ctx, event)
}

// NewSQSChoreographyEventRouter creates a new event router for SQS/SNS choreography
func NewSQSChoreographyEventRouter() *SQSChoreographyEventRouter {
	return &SQSChoreographyEventRouter{
		handlers: make(map[string][]SQSEventHandler),
	}
}

// RegisterHandler registers an event handler for a specific event type
func (r *SQSChoreographyEventRouter) RegisterHandler(eventType string, handler SQSEventHandler) {
	r.handlers[eventType] = append(r.handlers[eventType], handler)
}

// HandlerID implements the infrastructure.EventHandler interface
func (r *SQSChoreographyEventRouter) HandlerID() string {
	return "choreography-event-router"
}

// Handle implements the infrastructure.EventHandler interface
func (r *SQSChoreographyEventRouter) Handle(ctx context.Context, event *events.Event) error {
	handlers, exists := r.handlers[event.Topic.String()]
	if !exists {
		fmt.Printf("No handlers registered for event type: %s\n", event.Topic.String())
		return nil
	}

	for _, handler := range handlers {
		if err := handler.Handle(ctx, event); err != nil {
			fmt.Printf("Handler failed for event %s: %v\n", event.Topic.String(), err)
			// In a production system, you might want to publish a failure event
			// or implement retry logic
		}
	}

	return nil
}

// Payment Service Event Handlers (SQS/SNS Choreography)

// SQSPaymentInitiatedHandler handles PaymentInitiated events
type SQSPaymentInitiatedHandler struct {
	eventPublisher events.Publisher
}

func NewSQSPaymentInitiatedHandler(eventPublisher events.Publisher) *SQSPaymentInitiatedHandler {
	return &SQSPaymentInitiatedHandler{eventPublisher: eventPublisher}
}

func (h *SQSPaymentInitiatedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != events.PaymentCreatedEvent {
		return nil
	}

	// Extract payment data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentMethod, ok := data["payment_method"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid payment method in payload")
	}

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
		)

		return h.eventPublisher.Publish(ctx, walletDebitEvent)
	}

	// For non-wallet payments, go directly to gateway processing
	gatewayEvent := events.NewEvent(
		models.ID(data["payment_id"].(string)),
		"gateway.processing.requested",
		map[string]interface{}{
			"payment_id": data["payment_id"],
			"amount":     data["amount"],
			"currency":   data["currency"],
			"gateway":    "stripe", // Default gateway
		},
	)

	return h.eventPublisher.Publish(ctx, gatewayEvent)
}

// SQSWalletDebitedHandler handles WalletDebited events to continue payment flow
type SQSWalletDebitedHandler struct {
	eventPublisher events.Publisher
}

func NewSQSWalletDebitedHandler(eventPublisher events.Publisher) *SQSWalletDebitedHandler {
	return &SQSWalletDebitedHandler{eventPublisher: eventPublisher}
}

func (h *SQSWalletDebitedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != events.WalletDebitedEvent {
		return nil
	}

	// Extract data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentID, ok := data["payment_id"].(string)
	if !ok {
		return fmt.Errorf("payment_id not found in payload")
	}

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
	)

	return h.eventPublisher.Publish(ctx, gatewayEvent)
}

// SQSInsufficientFundsHandler handles InsufficientFunds events to fail payment
type SQSInsufficientFundsHandler struct {
	eventPublisher events.Publisher
}

func NewSQSInsufficientFundsHandler(eventPublisher events.Publisher) *SQSInsufficientFundsHandler {
	return &SQSInsufficientFundsHandler{eventPublisher: eventPublisher}
}

func (h *SQSInsufficientFundsHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != events.InsufficientFundsEvent {
		return nil
	}

	// Extract data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentID, ok := data["payment_id"].(string)
	if !ok {
		return fmt.Errorf("payment_id not found in payload")
	}

	// Fail the payment due to insufficient funds
	paymentFailEvent := events.NewEvent(
		models.ID(paymentID),
		"payment.failure.requested",
		map[string]interface{}{
			"payment_id": paymentID,
			"reason":     "Insufficient funds in wallet",
			"error_code": "INSUFFICIENT_FUNDS",
		},
	)

	return h.eventPublisher.Publish(ctx, paymentFailEvent)
}

// SQSGatewayProcessingCompletedHandler handles successful gateway processing
type SQSGatewayProcessingCompletedHandler struct {
	eventPublisher events.Publisher
}

func NewSQSGatewayProcessingCompletedHandler(eventPublisher events.Publisher) *SQSGatewayProcessingCompletedHandler {
	return &SQSGatewayProcessingCompletedHandler{eventPublisher: eventPublisher}
}

func (h *SQSGatewayProcessingCompletedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != "gateway.processing.completed" {
		return nil
	}

	// Extract data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentID, ok := data["payment_id"].(string)
	if !ok {
		return fmt.Errorf("payment_id not found in payload")
	}

	gatewayTransactionID, ok := data["gateway_transaction_id"].(string)
	if !ok {
		gatewayTransactionID = models.GenerateUUID().String()
	}

	// Complete the payment
	paymentCompleteEvent := events.NewEvent(
		models.ID(paymentID),
		"payment.completion.requested",
		map[string]interface{}{
			"payment_id":             paymentID,
			"gateway_transaction_id": gatewayTransactionID,
			"transaction_id":         models.GenerateUUID().String(),
		},
	)

	return h.eventPublisher.Publish(ctx, paymentCompleteEvent)
}

// SQSGatewayProcessingFailedHandler handles failed gateway processing
type SQSGatewayProcessingFailedHandler struct {
	eventPublisher events.Publisher
}

func NewSQSGatewayProcessingFailedHandler(eventPublisher events.Publisher) *SQSGatewayProcessingFailedHandler {
	return &SQSGatewayProcessingFailedHandler{eventPublisher: eventPublisher}
}

func (h *SQSGatewayProcessingFailedHandler) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != "gateway.processing.failed" {
		return nil
	}

	// Extract data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentID, ok := data["payment_id"].(string)
	if !ok {
		return fmt.Errorf("payment_id not found in payload")
	}

	gatewayError, ok := data["error"].(string)
	if !ok {
		gatewayError = "Unknown gateway error"
	}

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
		)

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
	)

	return h.eventPublisher.Publish(ctx, paymentFailEvent)
}

// MockGatewayService simulates external gateway processing
type MockGatewayService struct {
	eventPublisher events.Publisher
}

func NewMockGatewayService(eventPublisher events.Publisher) *MockGatewayService {
	return &MockGatewayService{eventPublisher: eventPublisher}
}

func (s *MockGatewayService) Handle(ctx context.Context, event *events.Event) error {
	if event.Topic.String() != "gateway.processing.requested" {
		return nil
	}

	fmt.Printf("Mock Gateway: Processing payment request: %+v\n", event)

	// Extract data
	var data map[string]interface{}
	if err := event.UnmarshalPayload(&data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	paymentID, ok := data["payment_id"].(string)
	if !ok {
		return fmt.Errorf("payment_id not found in payload")
	}

	// Simulate processing (90% success rate)
	// In real implementation, this would call actual payment gateway
	gatewayTransactionID := models.GenerateUUID().String()

	// Simulate success for demo
	gatewayCompleteEvent := events.NewEvent(
		models.ID(paymentID),
		"gateway.processing.completed",
		map[string]interface{}{
			"payment_id":             paymentID,
			"gateway_transaction_id": gatewayTransactionID,
			"gateway":                "stripe",
			"status":                 "success",
		},
	)

	fmt.Printf("Mock Gateway: Payment processed successfully: %s\n", paymentID)
	return s.eventPublisher.Publish(ctx, gatewayCompleteEvent)
}
