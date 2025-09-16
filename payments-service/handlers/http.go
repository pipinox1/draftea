package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/draftea/payment-system/payments-service/application"
	"github.com/go-chi/chi/v5"
)

// PaymentHandlers contains payment HTTP handlers
type PaymentHandlers struct {
	createPayment *application.CreatePaymentChoreography
	getPayment    *application.GetPayment
}

// NewPaymentHandlers creates new payment handlers
func NewPaymentHandlers(
	createPayment *application.CreatePaymentChoreography,
	getPayment *application.GetPayment,
) *PaymentHandlers {
	return &PaymentHandlers{
		createPayment: createPayment,
		getPayment:    getPayment,
	}
}

// CreatePayment handles payment creation requests
func (h *PaymentHandlers) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreatePaymentCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response, err := h.createPayment.Execute(r.Context(), &cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetPayment handles payment retrieval requests
func (h *PaymentHandlers) GetPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		http.Error(w, "Payment ID is required", http.StatusBadRequest)
		return
	}

	query := &application.GetPaymentQuery{
		PaymentID: paymentID,
	}

	response, err := h.getPayment.Execute(r.Context(), query)
	if err != nil {
		if err.Error() == "payment not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers payment routes
func (h *PaymentHandlers) RegisterRoutes(r chi.Router) {
	r.Route("/payments", func(r chi.Router) {
		r.Post("/", h.CreatePayment)
		r.Get("/{id}", h.GetPayment)
	})
}