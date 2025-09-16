package application

import (
	"context"
	"testing"

	"github.com/draftea/payment-system/payments-service/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleExternalWebhooks_Execute(t *testing.T) {
	validPaymentID := "550e8400-e29b-41d4-a716-446655440020"

	tests := []struct {
		name          string
		command       *HandleExternalWebhooksCommand
		setupMocks    func(*mocks.MockPublisher)
		expectedError string
	}{
		{
			name: "successful stripe webhook processing",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload: []byte(`{
					"type": "payment_intent.succeeded",
					"data": {
						"object": {
							"id": "pi_1234567890",
							"amount": 5000,
							"currency": "usd",
							"status": "succeeded",
							"metadata": {
								"payment_reference": "` + validPaymentID + `"
							}
						}
					}
				}`),
				Signature: "",
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "successful external gateway webhook processing",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload: []byte(`{
					"event_type": "payment.completed",
					"transaction_id": "txn_1234567890",
					"external_id": "ext_123",
					"payment_reference": "` + validPaymentID + `",
					"amount": 10000,
					"currency": "USD",
					"status": "completed",
					"timestamp": "2023-01-15T10:30:00Z"
				}`),
				Signature: "",
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "empty provider",
			command: &HandleExternalWebhooksCommand{
				Provider: "",
				Payload:  []byte(`{"test": "data"}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "provider is required",
		},
		{
			name: "empty payload",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  []byte(``),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "payload is required",
		},
		{
			name: "nil payload",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  nil,
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "payload is required",
		},
		{
			name: "unsupported provider",
			command: &HandleExternalWebhooksCommand{
				Provider: "unsupported_provider",
				Payload:  []byte(`{"test": "data"}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail parsing
			},
			expectedError: "unsupported webhook provider",
		},
		{
			name: "invalid JSON payload for external gateway",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload:  []byte(`invalid json`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail parsing
			},
			expectedError: "failed to parse webhook payload",
		},
		{
			name: "invalid JSON payload for stripe",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  []byte(`invalid json`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail parsing
			},
			expectedError: "failed to parse webhook payload",
		},
		{
			name: "invalid payment reference",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload: []byte(`{
					"event_type": "payment.completed",
					"payment_reference": "invalid-uuid",
					"amount": 5000,
					"currency": "USD"
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				// No expectations - should fail UUID validation
			},
			expectedError: "invalid payment reference",
		},
		{
			name: "publisher error",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload: []byte(`{
					"event_type": "payment.completed",
					"payment_reference": "` + validPaymentID + `",
					"amount": 5000,
					"currency": "USD"
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).
					Return(errors.New("publisher error")).Once()
			},
			expectedError: "failed to publish external provider update event",
		},
		{
			name: "stripe webhook with failed payment",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload: []byte(`{
					"type": "payment_intent.payment_failed",
					"data": {
						"object": {
							"id": "pi_1234567890",
							"amount": 5000,
							"currency": "usd",
							"status": "payment_failed",
							"last_payment_error": {
								"code": "card_declined",
								"message": "Your card was declined."
							},
							"metadata": {
								"payment_reference": "` + validPaymentID + `"
							}
						}
					}
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "external gateway webhook with error",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload: []byte(`{
					"event_type": "payment.failed",
					"transaction_id": "txn_1234567890",
					"payment_reference": "` + validPaymentID + `",
					"amount": 5000,
					"currency": "USD",
					"status": "failed",
					"error_code": "insufficient_funds",
					"error_message": "Insufficient funds in account",
					"timestamp": "2023-01-15T10:30:00Z"
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "stripe webhook with minimal data",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload: []byte(`{
					"type": "payment_intent.created",
					"data": {
						"object": {
							"metadata": {
								"payment_reference": "` + validPaymentID + `"
							}
						}
					}
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "webhook with signature verification",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload: []byte(`{
					"type": "payment_intent.succeeded",
					"data": {
						"object": {
							"metadata": {
								"payment_reference": "` + validPaymentID + `"
							}
						}
					}
				}`),
				Signature: "test_signature",
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "external gateway webhook with metadata",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload: []byte(`{
					"event_type": "payment.completed",
					"payment_reference": "` + validPaymentID + `",
					"amount": 7500,
					"currency": "EUR",
					"status": "completed",
					"metadata": {
						"customer_id": "cust_123",
						"order_id": "order_456"
					}
				}`),
			},
			setupMocks: func(publisher *mocks.MockPublisher) {
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockPublisher := mocks.NewMockPublisher(t)
			tt.setupMocks(mockPublisher)

			// Create use case
			useCase := NewHandleExternalWebhooks(mockPublisher)

			// Execute
			err := useCase.Execute(context.Background(), tt.command)

			// Assertions
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleExternalWebhooks_parseWebhookPayload(t *testing.T) {
	useCase := &HandleExternalWebhooks{}
	validPaymentID := "550e8400-e29b-41d4-a716-446655440020"

	tests := []struct {
		name           string
		provider       string
		payload        []byte
		expectedError  string
		validateResult func(*ExternalWebhookPayload)
	}{
		{
			name:     "valid external gateway payload",
			provider: "external_gateway",
			payload: []byte(`{
				"event_type": "payment.completed",
				"transaction_id": "txn_123",
				"external_id": "ext_456",
				"payment_reference": "` + validPaymentID + `",
				"amount": 5000,
				"currency": "USD",
				"status": "completed",
				"timestamp": "2023-01-15T10:30:00Z"
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "external_gateway", result.Provider)
				assert.Equal(t, "payment.completed", result.EventType)
				assert.Equal(t, "txn_123", result.TransactionID)
				assert.Equal(t, "ext_456", result.ExternalID)
				assert.Equal(t, validPaymentID, result.PaymentReference)
				assert.Equal(t, int64(5000), result.Amount)
				assert.Equal(t, "USD", result.Currency)
				assert.Equal(t, "completed", result.Status)
			},
		},
		{
			name:     "valid stripe payload",
			provider: "stripe",
			payload: []byte(`{
				"type": "payment_intent.succeeded",
				"data": {
					"object": {
						"id": "pi_123",
						"amount": 7500,
						"currency": "eur",
						"status": "succeeded",
						"metadata": {
							"payment_reference": "` + validPaymentID + `"
						}
					}
				}
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "stripe", result.Provider)
				assert.Equal(t, "payment_intent.succeeded", result.EventType)
				assert.Equal(t, "pi_123", result.TransactionID)
				assert.Equal(t, int64(7500), result.Amount)
				assert.Equal(t, "eur", result.Currency)
				assert.Equal(t, "succeeded", result.Status)
				assert.Equal(t, validPaymentID, result.PaymentReference)
			},
		},
		{
			name:          "unsupported provider",
			provider:      "unsupported",
			payload:       []byte(`{"test": "data"}`),
			expectedError: "unsupported webhook provider",
		},
		{
			name:          "invalid JSON for external gateway",
			provider:      "external_gateway",
			payload:       []byte(`invalid json`),
			expectedError: "failed to parse external gateway webhook",
		},
		{
			name:          "invalid JSON for stripe",
			provider:      "stripe",
			payload:       []byte(`invalid json`),
			expectedError: "failed to parse Stripe webhook",
		},
		{
			name:     "stripe payload missing metadata",
			provider: "stripe",
			payload: []byte(`{
				"type": "payment_intent.created",
				"data": {
					"object": {
						"id": "pi_123"
					}
				}
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "stripe", result.Provider)
				assert.Equal(t, "payment_intent.created", result.EventType)
				assert.Equal(t, "pi_123", result.TransactionID)
				assert.Empty(t, result.PaymentReference)
			},
		},
		{
			name:     "external gateway with error fields",
			provider: "external_gateway",
			payload: []byte(`{
				"event_type": "payment.failed",
				"payment_reference": "` + validPaymentID + `",
				"amount": 2500,
				"currency": "GBP",
				"status": "failed",
				"error_code": "card_declined",
				"error_message": "Card was declined by issuer"
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "external_gateway", result.Provider)
				assert.Equal(t, "payment.failed", result.EventType)
				assert.Equal(t, validPaymentID, result.PaymentReference)
				assert.Equal(t, int64(2500), result.Amount)
				assert.Equal(t, "GBP", result.Currency)
				assert.Equal(t, "failed", result.Status)
				assert.Equal(t, "card_declined", result.ErrorCode)
				assert.Equal(t, "Card was declined by issuer", result.ErrorMessage)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := useCase.parseWebhookPayload(tt.provider, tt.payload)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validateResult != nil {
					tt.validateResult(result)
				}
			}
		})
	}
}

func TestHandleExternalWebhooks_parseStripeWebhook(t *testing.T) {
	useCase := &HandleExternalWebhooks{}
	validPaymentID := "550e8400-e29b-41d4-a716-446655440020"

	tests := []struct {
		name           string
		payload        []byte
		expectedError  string
		validateResult func(*ExternalWebhookPayload)
	}{
		{
			name: "complete stripe webhook",
			payload: []byte(`{
				"type": "payment_intent.succeeded",
				"data": {
					"object": {
						"id": "pi_1234567890",
						"amount": 5000,
						"currency": "usd",
						"status": "succeeded",
						"metadata": {
							"payment_reference": "` + validPaymentID + `",
							"custom_field": "custom_value"
						}
					}
				}
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "payment_intent.succeeded", result.EventType)
				assert.Equal(t, "pi_1234567890", result.TransactionID)
				assert.Equal(t, int64(5000), result.Amount)
				assert.Equal(t, "usd", result.Currency)
				assert.Equal(t, "succeeded", result.Status)
				assert.Equal(t, validPaymentID, result.PaymentReference)
			},
		},
		{
			name: "stripe webhook with partial data",
			payload: []byte(`{
				"type": "payment_intent.created",
				"data": {
					"object": {
						"id": "pi_partial"
					}
				}
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "payment_intent.created", result.EventType)
				assert.Equal(t, "pi_partial", result.TransactionID)
				assert.Equal(t, int64(0), result.Amount)
				assert.Empty(t, result.Currency)
				assert.Empty(t, result.Status)
				assert.Empty(t, result.PaymentReference)
			},
		},
		{
			name:          "invalid JSON",
			payload:       []byte(`invalid json`),
			expectedError: "invalid character",
		},
		{
			name: "missing data object",
			payload: []byte(`{
				"type": "payment_intent.succeeded"
			}`),
			expectedError: "",
			validateResult: func(result *ExternalWebhookPayload) {
				assert.Equal(t, "payment_intent.succeeded", result.EventType)
				assert.Empty(t, result.TransactionID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var webhookData ExternalWebhookPayload
			err := useCase.parseStripeWebhook(tt.payload, &webhookData)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				if tt.validateResult != nil {
					tt.validateResult(&webhookData)
				}
			}
		})
	}
}

func TestHandleExternalWebhooks_validateCommand(t *testing.T) {
	useCase := &HandleExternalWebhooks{}

	tests := []struct {
		name          string
		command       *HandleExternalWebhooksCommand
		expectedError string
	}{
		{
			name: "valid command",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  []byte(`{"test": "data"}`),
				Signature: "optional_signature",
			},
			expectedError: "",
		},
		{
			name: "valid command without signature",
			command: &HandleExternalWebhooksCommand{
				Provider: "external_gateway",
				Payload:  []byte(`{"test": "data"}`),
			},
			expectedError: "",
		},
		{
			name: "empty provider",
			command: &HandleExternalWebhooksCommand{
				Provider: "",
				Payload:  []byte(`{"test": "data"}`),
			},
			expectedError: "provider is required",
		},
		{
			name: "empty payload",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  []byte(``),
			},
			expectedError: "payload is required",
		},
		{
			name: "nil payload",
			command: &HandleExternalWebhooksCommand{
				Provider: "stripe",
				Payload:  nil,
			},
			expectedError: "payload is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := useCase.validateCommand(tt.command)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleExternalWebhooks_verifyWebhookSignature(t *testing.T) {
	useCase := &HandleExternalWebhooks{}
	payload := []byte(`{"test": "data"}`)

	tests := []struct {
		name          string
		provider      string
		signature     string
		expectedError string
	}{
		{
			name:          "no signature provided",
			provider:      "stripe",
			signature:     "",
			expectedError: "",
		},
		{
			name:          "stripe with signature",
			provider:      "stripe",
			signature:     "test_signature",
			expectedError: "",
		},
		{
			name:          "external gateway with signature",
			provider:      "external_gateway",
			signature:     "test_signature",
			expectedError: "",
		},
		{
			name:          "unsupported provider with signature",
			provider:      "unsupported",
			signature:     "test_signature",
			expectedError: "unsupported provider for signature verification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := useCase.verifyWebhookSignature(tt.provider, payload, tt.signature)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}