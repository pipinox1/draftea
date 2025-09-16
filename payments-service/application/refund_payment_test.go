package application

import (
	"context"
	"testing"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/payments-service/mocks"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRefundPayment_Execute(t *testing.T) {
	// Test data
	validPaymentID := models.ID("550e8400-e29b-41d4-a716-446655440020")
	validUserID := models.ID("550e8400-e29b-41d4-a716-446655440010")
	validRequestedBy := models.ID("550e8400-e29b-41d4-a716-446655440030")

	completedPayment := &domain.Payment{
		ID:     validPaymentID,
		UserID: validUserID,
		Amount: models.NewMoney(10000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeWallet,
			WalletPaymentMethod: &domain.WalletPaymentMethod{
				WalletID: "550e8400-e29b-41d4-a716-446655440001",
			},
		},
		Description: "Test payment",
		Status:      domain.PaymentStatusCompleted,
		Timestamps:  models.NewTimestamps(),
	}

	tests := []struct {
		name           string
		command        *RefundPaymentCommand
		setupMocks     func(*mocks.MockPaymentRepository, *mocks.MockPublisher)
		expectedError  string
		validateResult func(*RefundPaymentResponse)
	}{
		{
			name: "successful full refund",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{}, // Empty means full refund
				Reason:      "Customer requested refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.PaymentRefundInitiatedEvent
				})).Return(nil).Once()
			},
			expectedError: "",
			validateResult: func(result *RefundPaymentResponse) {
				assert.Equal(t, validPaymentID, result.PaymentID)
				assert.NotEmpty(t, result.RefundID)
				assert.Equal(t, int64(10000), result.Amount.Amount)
				assert.Equal(t, "USD", result.Amount.Currency)
				assert.Equal(t, "initiated", result.Status)
			},
		},
		{
			name: "successful partial refund",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(5000, "USD"),
				Reason:      "Partial refund requested",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.PaymentRefundInitiatedEvent
				})).Return(nil).Once()
			},
			expectedError: "",
			validateResult: func(result *RefundPaymentResponse) {
				assert.Equal(t, validPaymentID, result.PaymentID)
				assert.NotEmpty(t, result.RefundID)
				assert.Equal(t, int64(5000), result.Amount.Amount)
				assert.Equal(t, "USD", result.Amount.Currency)
				assert.Equal(t, "initiated", result.Status)
			},
		},
		{
			name: "empty payment ID",
			command: &RefundPaymentCommand{
				PaymentID:   models.ID(""),
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "payment ID is required",
		},
		{
			name: "empty reason",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "reason is required",
		},
		{
			name: "empty requested by",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: models.ID(""),
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "requested by user ID is required",
		},
		{
			name: "payment not found",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(nil, nil).Once()
			},
			expectedError: "payment not found",
		},
		{
			name: "repository error",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).
					Return(nil, errors.New("database error")).Once()
			},
			expectedError: "failed to find payment",
		},
		{
			name: "payment not completed",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				incompletePayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(10000, "USD"),
					Status: domain.PaymentStatusProcessing, // Not completed
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(incompletePayment, nil).Once()
			},
			expectedError: "only completed payments can be refunded",
		},
		{
			name: "refund amount exceeds payment amount",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(15000, "USD"), // More than payment amount
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
			},
			expectedError: "refund amount cannot exceed payment amount",
		},
		{
			name: "refund currency mismatch",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(5000, "EUR"), // Different currency
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
			},
			expectedError: "refund currency must match payment currency",
		},
		{
			name: "negative refund amount",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(-1000, "USD"),
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
			},
			expectedError: "refund amount must be positive",
		},
		{
			name: "publisher error",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(completedPayment, nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).
					Return(errors.New("publisher error")).Once()
			},
			expectedError: "failed to publish refund initiated event",
		},
		{
			name: "amount specified without currency",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{Amount: 5000, Currency: ""}, // No currency
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "currency is required when amount is specified",
		},
		{
			name: "refund credit card payment",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(7500, "USD"),
				Reason:      "Product defective",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				cardPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(10000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeCreditCard,
						CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
							CardToken: "tok_1234567890",
						},
					},
					Status: domain.PaymentStatusCompleted,
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(cardPayment, nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
			validateResult: func(result *RefundPaymentResponse) {
				assert.Equal(t, validPaymentID, result.PaymentID)
				assert.NotEmpty(t, result.RefundID)
				assert.Equal(t, int64(7500), result.Amount.Amount)
				assert.Equal(t, "USD", result.Amount.Currency)
				assert.Equal(t, "initiated", result.Status)
			},
		},
		{
			name: "failed payment cannot be refunded",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				failedPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(10000, "USD"),
					Status: domain.PaymentStatusFailed, // Failed status
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(failedPayment, nil).Once()
			},
			expectedError: "only completed payments can be refunded",
		},
		{
			name: "cancelled payment cannot be refunded",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test refund",
				RequestedBy: validRequestedBy,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				cancelledPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(10000, "USD"),
					Status: domain.PaymentStatusCancelled, // Cancelled status
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(cancelledPayment, nil).Once()
			},
			expectedError: "only completed payments can be refunded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRepo := mocks.NewMockPaymentRepository(t)
			mockPublisher := mocks.NewMockPublisher(t)

			tt.setupMocks(mockRepo, mockPublisher)

			// Create use case
			useCase := NewRefundPayment(mockRepo, mockPublisher)

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
				if tt.validateResult != nil {
					tt.validateResult(result)
				}
			}
		})
	}
}

func TestRefundPayment_validateCommand(t *testing.T) {
	useCase := &RefundPayment{}
	validPaymentID := models.ID("550e8400-e29b-41d4-a716-446655440020")
	validRequestedBy := models.ID("550e8400-e29b-41d4-a716-446655440030")

	tests := []struct {
		name          string
		command       *RefundPaymentCommand
		expectedError string
	}{
		{
			name: "valid command - full refund",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Customer request",
				RequestedBy: validRequestedBy,
			},
			expectedError: "",
		},
		{
			name: "valid command - partial refund",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.NewMoney(5000, "USD"),
				Reason:      "Partial refund",
				RequestedBy: validRequestedBy,
			},
			expectedError: "",
		},
		{
			name: "empty payment ID",
			command: &RefundPaymentCommand{
				PaymentID:   models.ID(""),
				Amount:      models.Money{},
				Reason:      "Test reason",
				RequestedBy: validRequestedBy,
			},
			expectedError: "payment ID is required",
		},
		{
			name: "empty reason",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "",
				RequestedBy: validRequestedBy,
			},
			expectedError: "reason is required",
		},
		{
			name: "empty requested by",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{},
				Reason:      "Test reason",
				RequestedBy: models.ID(""),
			},
			expectedError: "requested by user ID is required",
		},
		{
			name: "amount without currency",
			command: &RefundPaymentCommand{
				PaymentID:   validPaymentID,
				Amount:      models.Money{Amount: 5000, Currency: ""},
				Reason:      "Test reason",
				RequestedBy: validRequestedBy,
			},
			expectedError: "currency is required when amount is specified",
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

func TestRefundPayment_validateRefundEligibility(t *testing.T) {
	useCase := &RefundPayment{}

	completedPayment := &domain.Payment{
		Amount: models.NewMoney(10000, "USD"),
		Status: domain.PaymentStatusCompleted,
	}

	tests := []struct {
		name          string
		payment       *domain.Payment
		refundAmount  models.Money
		expectedError string
	}{
		{
			name:          "valid full refund",
			payment:       completedPayment,
			refundAmount:  models.Money{}, // Zero amount means full refund
			expectedError: "",
		},
		{
			name:          "valid partial refund",
			payment:       completedPayment,
			refundAmount:  models.NewMoney(5000, "USD"),
			expectedError: "",
		},
		{
			name: "payment not completed",
			payment: &domain.Payment{
				Amount: models.NewMoney(10000, "USD"),
				Status: domain.PaymentStatusProcessing,
			},
			refundAmount:  models.Money{},
			expectedError: "only completed payments can be refunded",
		},
		{
			name:          "refund amount exceeds payment",
			payment:       completedPayment,
			refundAmount:  models.NewMoney(15000, "USD"),
			expectedError: "refund amount cannot exceed payment amount",
		},
		{
			name:          "currency mismatch",
			payment:       completedPayment,
			refundAmount:  models.NewMoney(5000, "EUR"),
			expectedError: "refund currency must match payment currency",
		},
		{
			name:          "negative refund amount",
			payment:       completedPayment,
			refundAmount:  models.NewMoney(-1000, "USD"),
			expectedError: "refund amount must be positive",
		},
		{
			name: "failed payment",
			payment: &domain.Payment{
				Amount: models.NewMoney(10000, "USD"),
				Status: domain.PaymentStatusFailed,
			},
			refundAmount:  models.Money{},
			expectedError: "only completed payments can be refunded",
		},
		{
			name: "cancelled payment",
			payment: &domain.Payment{
				Amount: models.NewMoney(10000, "USD"),
				Status: domain.PaymentStatusCancelled,
			},
			refundAmount:  models.Money{},
			expectedError: "only completed payments can be refunded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := useCase.validateRefundEligibility(tt.payment, tt.refundAmount)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}