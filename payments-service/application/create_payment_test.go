package application

import (
	"context"
	"testing"

	"github.com/draftea/payment-system/payments-service/mocks"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreatePaymentChoreography_Execute(t *testing.T) {
	tests := []struct {
		name           string
		command        *CreatePaymentCommand
		setupMocks     func(*mocks.MockPaymentRepository, *mocks.MockPublisher)
		expectedError  string
		expectedResult *CreatePaymentResponse
	}{
		{
			name: "successful wallet payment creation",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.PaymentCreatedEvent
				})).Return(nil).Once()
			},
			expectedError: "",
			expectedResult: &CreatePaymentResponse{
				PaymentID: "", // Will be set by the domain
			},
		},
		{
			name: "successful credit card payment creation",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            10000,
				Currency:          "USD",
				PaymentMethodType: "credit_card",
				CardToken:         stringPtr("tok_1234567890"),
				Description:       "Credit card payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.PaymentCreatedEvent
				})).Return(nil).Once()
			},
			expectedError: "",
			expectedResult: &CreatePaymentResponse{
				PaymentID: "",
			},
		},
		{
			name: "invalid user ID",
			command: &CreatePaymentCommand{
				UserID:            "invalid-uuid",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail before calling mocks
			},
			expectedError:  "invalid user ID",
			expectedResult: nil,
		},
		{
			name: "negative amount",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            -1000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "amount must be positive",
			expectedResult: nil,
		},
		{
			name: "wallet payment without wallet ID",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "wallet ID is required for wallet payments",
			expectedResult: nil,
		},
		{
			name: "credit card payment without card token",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "credit_card",
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "card token is required for card payments",
			expectedResult: nil,
		},
		{
			name: "repository save error",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).
					Return(errors.New("database error")).Once()
			},
			expectedError:  "failed to save payment",
			expectedResult: nil,
		},
		{
			name: "event publisher error",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).
					Return(errors.New("publisher error")).Once()
			},
			expectedError:  "failed to publish events",
			expectedResult: nil,
		},
		{
			name: "invalid payment method type",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "invalid_type",
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "invalid payment method type",
			expectedResult: nil,
		},
		{
			name: "empty currency",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "currency is required",
			expectedResult: nil,
		},
		{
			name: "empty user ID",
			command: &CreatePaymentCommand{
				UserID:            "",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError:  "user ID is required",
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRepo := mocks.NewMockPaymentRepository(t)
			mockPublisher := mocks.NewMockPublisher(t)

			tt.setupMocks(mockRepo, mockPublisher)

			// Create use case
			useCase := NewCreatePaymentChoreography(mockRepo, mockPublisher)

			// Execute
			result, err := useCase.Execute(context.Background(), tt.command)

			// Assertions
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.PaymentID)

				// Verify the payment ID is a valid UUID
				_, err := models.NewID(result.PaymentID)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreatePaymentChoreography_validateCommand(t *testing.T) {
	useCase := &CreatePaymentChoreography{}

	tests := []struct {
		name          string
		command       *CreatePaymentCommand
		expectedError string
	}{
		{
			name: "valid wallet command",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
				Description:       "Test payment",
			},
			expectedError: "",
		},
		{
			name: "valid credit card command",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "credit_card",
				CardToken:         stringPtr("tok_1234567890"),
				Description:       "Test payment",
			},
			expectedError: "",
		},
		{
			name: "empty user ID",
			command: &CreatePaymentCommand{
				UserID:            "",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
			},
			expectedError: "user ID is required",
		},
		{
			name: "zero amount",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            0,
				Currency:          "USD",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
			},
			expectedError: "amount must be positive",
		},
		{
			name: "empty currency",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "",
				PaymentMethodType: "wallet",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
			},
			expectedError: "currency is required",
		},
		{
			name: "empty payment method type",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "",
				WalletID:          stringPtr("550e8400-e29b-41d4-a716-446655440001"),
			},
			expectedError: "payment method type is required",
		},
		{
			name: "wallet payment missing wallet ID",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "wallet",
			},
			expectedError: "wallet ID is required for wallet payments",
		},
		{
			name: "credit card payment missing card token",
			command: &CreatePaymentCommand{
				UserID:            "550e8400-e29b-41d4-a716-446655440010",
				Amount:            5000,
				Currency:          "USD",
				PaymentMethodType: "credit_card",
			},
			expectedError: "card token is required for card payments",
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

// stringPtr is a helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}