package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/draftea/payment-system/payments-service/application"
	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// PaymentEventHandlers handles all payment-related events in the choreography
type PaymentEventHandlers struct {
	processPaymentMethod           *application.ProcessPaymentMethod
	processWalletDebit             *application.ProcessWalletDebit
	handleExternalWebhooks         *application.HandleExternalWebhooks
	processExternalProviderUpdates *application.ProcessExternalProviderUpdates
	processPaymentOperationResult  *application.ProcessPaymentOperationResult
	processPaymentInconsistentOp   *application.ProcessPaymentInconsistentOperation
	refundPayment                  *application.RefundPayment
	processRefund                  *application.ProcessRefund
}

// Handle implements the events.EventHandler interface
func (h *PaymentEventHandlers) Handle(ctx context.Context, event *events.Event) error {
	switch event.EventType {
	case events.PaymentCreatedEvent:
		return h.HandlePaymentInitiated(ctx, event)
	case events.WalletDebitedEvent:
		return h.HandleWalletDebited(ctx, event)
	case events.InsufficientFundsEvent:
		return h.HandleInsufficientFunds(ctx, event)
	case events.ExternalProviderUpdateEvent:
		return h.HandleExternalProviderUpdate(ctx, event)
	case events.PaymentOperationCompletedEvent:
		return h.HandlePaymentOperationCompleted(ctx, event)
	case events.PaymentOperationFailedEvent:
		return h.HandlePaymentOperationFailed(ctx, event)
	case events.PaymentInconsistentStateEvent:
		return h.HandlePaymentInconsistentState(ctx, event)
	case events.PaymentRefundInitiatedEvent:
		return h.HandlePaymentRefundInitiated(ctx, event)
	default:
		// Unknown event type, ignore
		return nil
	}
}

// HandlerID returns the unique identifier for this event handler
func (h *PaymentEventHandlers) HandlerID() string {
	return "payment-service-event-handler"
}

// NewPaymentEventHandlers creates new payment event handlers
func NewPaymentEventHandlers(
	processPaymentMethod *application.ProcessPaymentMethod,
	processWalletDebit *application.ProcessWalletDebit,
	handleExternalWebhooks *application.HandleExternalWebhooks,
	processExternalProviderUpdates *application.ProcessExternalProviderUpdates,
	processPaymentOperationResult *application.ProcessPaymentOperationResult,
	processPaymentInconsistentOp *application.ProcessPaymentInconsistentOperation,
	refundPayment *application.RefundPayment,
	processRefund *application.ProcessRefund,
) *PaymentEventHandlers {
	return &PaymentEventHandlers{
		processPaymentMethod:           processPaymentMethod,
		processWalletDebit:             processWalletDebit,
		handleExternalWebhooks:         handleExternalWebhooks,
		processExternalProviderUpdates: processExternalProviderUpdates,
		processPaymentOperationResult:  processPaymentOperationResult,
		processPaymentInconsistentOp:   processPaymentInconsistentOp,
		refundPayment:                  refundPayment,
		processRefund:                  processRefund,
	}
}

// HandlePaymentInitiated handles payment initiated events
func (h *PaymentEventHandlers) HandlePaymentInitiated(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentCreatedEvent {
		return nil
	}

	// Extract payment ID from event
	var data PaymentInitiatedData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse payment initiated data")
	}

	// Process payment method
	cmd := &application.ProcessPaymentMethodCommand{
		PaymentID: data.PaymentID,
	}

	if err := h.processPaymentMethod.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process payment method for payment %s: %v\n", data.PaymentID, err)
		return nil // Don't return error to avoid retries - inconsistent operation handler will catch this
	}

	return nil
}

// HandleWalletDebited handles wallet debited events from wallet service
func (h *PaymentEventHandlers) HandleWalletDebited(ctx context.Context, event *events.Event) error {
	if event.EventType != events.WalletDebitedEvent {
		return nil
	}

	var data WalletDebitedData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse wallet debited data")
	}

	// Process wallet debit result
	cmd := &application.ProcessWalletDebitCommand{
		PaymentID:     data.PaymentID,
		WalletID:      data.WalletID.String(),
		TransactionID: data.TransactionID.String(),
		Amount:        data.Amount,
		Status:        "completed",
	}

	if err := h.processWalletDebit.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process wallet debit for payment %s: %v\n", data.PaymentID, err)
		return nil
	}

	return nil
}

// HandleInsufficientFunds handles insufficient funds events from wallet service
func (h *PaymentEventHandlers) HandleInsufficientFunds(ctx context.Context, event *events.Event) error {
	if event.EventType != events.InsufficientFundsEvent {
		return nil
	}

	var data InsufficientFundsData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse insufficient funds data")
	}

	// Process wallet debit failure
	cmd := &application.ProcessWalletDebitCommand{
		PaymentID:    data.PaymentID,
		WalletID:     data.WalletID.String(),
		Amount:       data.RequestedAmount,
		Status:       "failed",
		ErrorCode:    "insufficient_funds",
		ErrorMessage: fmt.Sprintf("Insufficient funds. Requested: %d, Available: %d", data.RequestedAmount.Amount, data.AvailableBalance.Amount),
	}

	if err := h.processWalletDebit.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process wallet debit failure for payment %s: %v\n", data.PaymentID, err)
		return nil
	}

	return nil
}

// HandleExternalProviderUpdate handles external provider update events
func (h *PaymentEventHandlers) HandleExternalProviderUpdate(ctx context.Context, event *events.Event) error {
	if event.EventType != events.ExternalProviderUpdateEvent {
		return nil
	}

	var data application.ExternalProviderUpdateData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse external provider update data")
	}

	// Process external provider update
	cmd := &application.ProcessExternalProviderUpdatesCommand{
		Provider:         data.Provider,
		EventType:        data.EventType,
		TransactionID:    data.TransactionID,
		ExternalID:       data.ExternalID,
		PaymentReference: data.PaymentReference,
		Amount:           data.Amount,
		Status:           data.Status,
		ErrorCode:        data.ErrorCode,
		ErrorMessage:     data.ErrorMessage,
		Metadata:         data.Metadata,
	}

	if err := h.processExternalProviderUpdates.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process external provider update: %v\n", err)
		return nil
	}

	return nil
}

// HandlePaymentOperationCompleted handles payment operation completed events
func (h *PaymentEventHandlers) HandlePaymentOperationCompleted(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentOperationCompletedEvent {
		return nil
	}

	var data PaymentOperationCompletedData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse payment operation completed data")
	}

	// Process payment operation result
	cmd := &application.ProcessPaymentOperationResultCommand{
		OperationID:           data.OperationID,
		PaymentID:             data.PaymentID,
		Type:                  data.Type,
		Status:                domain.PaymentOperationStatusCompleted,
		Amount:                data.Amount,
		ProviderTransactionID: data.ProviderTransactionID,
		ExternalTransactionID: data.ExternalTransactionID,
	}

	if err := h.processPaymentOperationResult.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process payment operation result for payment %s: %v\n", data.PaymentID, err)
		return nil
	}

	return nil
}

// HandlePaymentOperationFailed handles payment operation failed events
func (h *PaymentEventHandlers) HandlePaymentOperationFailed(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentOperationFailedEvent {
		return nil
	}

	var data PaymentOperationFailedData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse payment operation failed data")
	}

	// Process payment operation result
	cmd := &application.ProcessPaymentOperationResultCommand{
		OperationID:  data.OperationID,
		PaymentID:    data.PaymentID,
		Type:         data.Type,
		Status:       domain.PaymentOperationStatusFailed,
		Amount:       data.Amount,
		ErrorCode:    data.ErrorCode,
		ErrorMessage: data.ErrorMessage,
	}

	if err := h.processPaymentOperationResult.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process payment operation failure for payment %s: %v\n", data.PaymentID, err)
		return nil
	}

	return nil
}

// HandlePaymentInconsistentState handles payment inconsistent state events
func (h *PaymentEventHandlers) HandlePaymentInconsistentState(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentInconsistentStateEvent {
		return nil
	}

	var data application.PaymentInconsistentStateData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse payment inconsistent state data")
	}

	// Process inconsistent payment
	cmd := &application.ProcessPaymentInconsistentOperationCommand{
		PaymentID:    data.PaymentID,
		Reason:       data.Reason,
		ErrorCode:    data.ErrorCode,
		ErrorMessage: data.ErrorMessage,
	}

	if err := h.processPaymentInconsistentOp.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process inconsistent payment %s: %v\n", data.PaymentID, err)
		return err // Return error for inconsistent operation failures
	}

	return nil
}

// HandlePaymentRefundInitiated handles payment refund initiated events
func (h *PaymentEventHandlers) HandlePaymentRefundInitiated(ctx context.Context, event *events.Event) error {
	if event.EventType != events.PaymentRefundInitiatedEvent {
		return nil
	}

	var data application.PaymentRefundInitiatedData
	if err := h.parseEventData(event, &data); err != nil {
		return errors.Wrap(err, "failed to parse payment refund initiated data")
	}

	// Process refund
	cmd := &application.ProcessRefundCommand{
		PaymentID:     data.PaymentID,
		RefundID:      data.RefundID,
		Amount:        data.Amount,
		Reason:        data.Reason,
		RequestedBy:   data.RequestedBy,
		PaymentMethod: data.PaymentMethod,
		UserID:        data.UserID,
	}

	if err := h.processRefund.Execute(ctx, cmd); err != nil {
		fmt.Printf("Failed to process refund for payment %s: %v\n", data.PaymentID, err)
		return nil
	}

	return nil
}

// parseEventData parses event data into the specified struct
func (h *PaymentEventHandlers) parseEventData(event *events.Event, target interface{}) error {
	// Convert event data to JSON and then to target struct
	jsonData, err := json.Marshal(event.Data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal event data")
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return errors.Wrap(err, "failed to unmarshal event data")
	}

	return nil
}

// Event data structures (imported from domain and other use cases)
type PaymentInitiatedData struct {
	PaymentID     models.ID            `json:"payment_id"`
	UserID        models.ID            `json:"user_id"`
	Amount        models.Money         `json:"amount"`
	PaymentMethod domain.PaymentMethod `json:"payment_method"`
	Description   string               `json:"description"`
}

type WalletDebitedData struct {
	WalletID      models.ID    `json:"wallet_id"`
	UserID        models.ID    `json:"user_id"`
	PaymentID     models.ID    `json:"payment_id"`
	TransactionID models.ID    `json:"transaction_id"`
	Amount        models.Money `json:"amount"`
	BalanceBefore models.Money `json:"balance_before"`
	BalanceAfter  models.Money `json:"balance_after"`
	Reference     string       `json:"reference"`
}

type InsufficientFundsData struct {
	WalletID         models.ID    `json:"wallet_id"`
	UserID           models.ID    `json:"user_id"`
	PaymentID        models.ID    `json:"payment_id"`
	RequestedAmount  models.Money `json:"requested_amount"`
	AvailableBalance models.Money `json:"available_balance"`
	Shortfall        models.Money `json:"shortfall"`
}

type PaymentOperationCompletedData struct {
	OperationID           models.ID                   `json:"operation_id"`
	PaymentID             models.ID                   `json:"payment_id"`
	Type                  domain.PaymentOperationType `json:"type"`
	Amount                models.Money                `json:"amount"`
	ProviderTransactionID string                      `json:"provider_transaction_id"`
	ExternalTransactionID string                      `json:"external_transaction_id"`
}

type PaymentOperationFailedData struct {
	OperationID  models.ID                   `json:"operation_id"`
	PaymentID    models.ID                   `json:"payment_id"`
	Type         domain.PaymentOperationType `json:"type"`
	Amount       models.Money                `json:"amount"`
	ErrorCode    string                      `json:"error_code"`
	ErrorMessage string                      `json:"error_message"`
}
