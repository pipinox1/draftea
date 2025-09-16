package handlers

import (
	"context"
	"fmt"
	"github.com/draftea/payment-system/wallet-service/application"

	"github.com/draftea/payment-system/shared/events"
	"github.com/pkg/errors"
)

// WalletEventHandlers contains event handlers for wallet service
type WalletEventHandlers struct {
	createMovement *application.CreateMovement
	revertMovement *application.RevertMovement
}

// NewWalletEventHandlers creates new wallet event handlers
func NewWalletEventHandlers(
	createMovement *application.CreateMovement,
	revertMovement *application.RevertMovement,
) *WalletEventHandlers {
	return &WalletEventHandlers{
		createMovement: createMovement,
		revertMovement: revertMovement,
	}
}

// Handle implements the events.EventHandler interface
func (h *WalletEventHandlers) Handle(ctx context.Context, event *events.Event) error {
	switch event.EventType {
	case events.WalletMovementCreationRequestedEvent:
		return h.HandleMovementCreationRequest(ctx, event)
	case events.WalletMovementRevertRequestedEvent:
		return h.HandleMovementRevertRequest(ctx, event)
	default:
		// Unknown event type, ignore
		return nil
	}
}

// HandlerID returns the unique identifier for this event handler
func (h *WalletEventHandlers) HandlerID() string {
	return "wallet-service-event-handler"
}

// HandleMovementCreationRequest handles movement creation requests
func (h *WalletEventHandlers) HandleMovementCreationRequest(ctx context.Context, event *events.Event) error {
	if event.EventType != events.WalletMovementCreationRequestedEvent {
		return nil
	}

	// Extract data from event
	data := event.Data.(map[string]interface{})

	// Validate required fields
	walletID, ok := data["wallet_id"].(string)
	if !ok {
		return errors.New("wallet_id is required")
	}

	movementType, ok := data["type"].(string)
	if !ok {
		return errors.New("movement type is required")
	}

	amount, ok := data["amount"].(float64)
	if !ok {
		return errors.New("amount is required")
	}

	currency, ok := data["currency"].(string)
	if !ok {
		return errors.New("currency is required")
	}

	reference, ok := data["reference"].(string)
	if !ok {
		return errors.New("reference is required")
	}

	// Extract optional fields
	var paymentID string
	if pid, exists := data["payment_id"].(string); exists {
		paymentID = pid
	}

	description, _ := data["description"].(string)

	// Create command
	cmd := &application.CreateMovementCommand{
		WalletID:    walletID,
		Type:        movementType,
		Amount:      int64(amount),
		Currency:    currency,
		Reference:   reference,
		PaymentID:   paymentID,
		Description: description,
	}

	// Execute create movement use case
	_, err := h.createMovement.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("Failed to create movement for wallet %s: %v\n", walletID, err)
		return err
	}

	return nil
}

// HandleMovementRevertRequest handles movement revert requests
func (h *WalletEventHandlers) HandleMovementRevertRequest(ctx context.Context, event *events.Event) error {
	if event.EventType != events.WalletMovementRevertRequestedEvent {
		return nil
	}

	// Extract data from event
	data := event.Data.(map[string]interface{})

	// Validate required fields
	movementID, ok := data["movement_id"].(string)
	if !ok {
		return errors.New("movement_id is required")
	}

	reason, ok := data["reason"].(string)
	if !ok {
		return errors.New("reason is required")
	}

	requestedBy, ok := data["requested_by"].(string)
	if !ok {
		return errors.New("requested_by is required")
	}

	// Create command
	cmd := &application.RevertMovementCommand{
		MovementID:  movementID,
		Reason:      reason,
		RequestedBy: requestedBy,
	}

	// Execute revert movement use case
	_, err := h.revertMovement.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("Failed to revert movement %s: %v\n", movementID, err)
		return err
	}

	return nil
}
