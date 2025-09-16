package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/draftea/payment-system/wallet-service/application"
	"github.com/go-chi/chi/v5"
)

// WalletHandlers contains wallet HTTP handlers
type WalletHandlers struct {
	getWallet      *application.GetWallet
	createMovement *application.CreateMovement
	revertMovement *application.RevertMovement
}

// NewWalletHandlers creates new wallet handlers
func NewWalletHandlers(
	getWallet *application.GetWallet,
	createMovement *application.CreateMovement,
	revertMovement *application.RevertMovement,
) *WalletHandlers {
	return &WalletHandlers{
		getWallet:      getWallet,
		createMovement: createMovement,
		revertMovement: revertMovement,
	}
}

// GetWallet handles wallet retrieval requests
func (h *WalletHandlers) GetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	userID := r.URL.Query().Get("user_id")

	if walletID == "" && userID == "" {
		http.Error(w, "Either wallet ID or user ID is required", http.StatusBadRequest)
		return
	}

	query := &application.GetWalletQuery{
		WalletID: walletID,
		UserID:   userID,
	}

	response, err := h.getWallet.Execute(r.Context(), query)
	if err != nil {
		if err.Error() == "wallet not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateMovement handles movement creation requests as per documentation API
func (h *WalletHandlers) CreateMovement(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		http.Error(w, "Wallet ID is required", http.StatusBadRequest)
		return
	}

	var cmd application.CreateMovementCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd.WalletID = walletID

	response, err := h.createMovement.Execute(r.Context(), &cmd)
	if err != nil {
		if err.Error() == "wallet not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "insufficient funds" {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RevertMovement handles movement revert requests as per documentation API
func (h *WalletHandlers) RevertMovement(w http.ResponseWriter, r *http.Request) {
	movementID := chi.URLParam(r, "movement_id")
	if movementID == "" {
		http.Error(w, "Movement ID is required", http.StatusBadRequest)
		return
	}

	var cmd application.RevertMovementCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd.MovementID = movementID

	response, err := h.revertMovement.Execute(r.Context(), &cmd)
	if err != nil {
		if err.Error() == "original transaction not found" || err.Error() == "movement not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "insufficient funds to revert this movement" {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers wallet routes
func (h *WalletHandlers) RegisterRoutes(r chi.Router) {
	// New API endpoints as per documentation
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/wallet/{id}", func(r chi.Router) {
			r.Get("/", h.GetWallet)
			r.Post("/movement", h.CreateMovement)
		})
		r.Route("/movement/{movement_id}", func(r chi.Router) {
			r.Post("/revert", h.RevertMovement)
		})
	})
}
