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

func TestProcessPaymentMethod_Execute(t *testing.T) {
	validPaymentID := models.ID("550e8400-e29b-41d4-a716-446655440020")
	validUserID := models.ID("550e8400-e29b-41d4-a716-446655440010")

	walletPayment := &domain.Payment{
		ID:     validPaymentID,
		UserID: validUserID,
		Amount: models.NewMoney(5000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeWallet,
			WalletPaymentMethod: &domain.WalletPaymentMethod{
				WalletID: "550e8400-e29b-41d4-a716-446655440001",
			},
		},
		Status:     domain.PaymentStatusInitiated,
		Timestamps: models.NewTimestamps(),
	}

	creditCardPayment := &domain.Payment{
		ID:     validPaymentID,
		UserID: validUserID,
		Amount: models.NewMoney(10000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeCreditCard,
			CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
				CardToken: "tok_1234567890",
			},
		},
		Status:     domain.PaymentStatusInitiated,
		Timestamps: models.NewTimestamps(),
	}

	tests := []struct {
		name          string
		command       *ProcessPaymentMethodCommand
		setupMocks    func(*mocks.MockPaymentRepository, *mocks.MockPublisher)
		expectedError string
	}{
		{
			name: "successful wallet payment processing",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(walletPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()

				// Expect wallet debit event
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.WalletDebitRequestedEvent
				})).Return(nil).Once()

				// Expect payment events (variadic arguments)
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "successful credit card payment processing",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(creditCardPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()

				// Expect payment operation events (variadic arguments)
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

				// Expect payment events (variadic arguments)
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "payment not found",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(nil, nil).Once()
			},
			expectedError: "payment not found",
		},
		{
			name: "repository find error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).
					Return(nil, errors.New("database error")).Once()
			},
			expectedError: "failed to find payment",
		},
		{
			name: "payment not in initiated status",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				processingPayment := &domain.Payment{
					ID:     validPaymentID,
					Status: domain.PaymentStatusProcessing, // Already processing
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(processingPayment, nil).Once()
			},
			expectedError: "payment must be in initiated status to process",
		},
		{
			name: "repository save error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				freshPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(5000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeWallet,
						WalletPaymentMethod: &domain.WalletPaymentMethod{
							WalletID: "550e8400-e29b-41d4-a716-446655440001",
						},
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(freshPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).
					Return(errors.New("save error")).Once()
			},
			expectedError: "failed to save payment",
		},
		{
			name: "wallet debit event publish error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				freshPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(5000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeWallet,
						WalletPaymentMethod: &domain.WalletPaymentMethod{
							WalletID: "550e8400-e29b-41d4-a716-446655440001",
						},
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(freshPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()

				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.WalletDebitRequestedEvent
				})).Return(errors.New("publish error")).Once()
			},
			expectedError: "failed to publish wallet debit requested event",
		},
		{
			name: "payment operation event publish error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				freshPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(10000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeCreditCard,
						CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
							CardToken: "tok_1234567890",
						},
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(freshPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()

				publisher.EXPECT().Publish(mock.Anything, mock.Anything).
					Return(errors.New("publish error")).Once()
			},
			expectedError: "failed to publish payment operation events",
		},
		{
			name: "payment events publish error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				freshPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(5000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeWallet,
						WalletPaymentMethod: &domain.WalletPaymentMethod{
							WalletID: "550e8400-e29b-41d4-a716-446655440001",
						},
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(freshPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()

				// First publish succeeds
				publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt *events.Event) bool {
					return evt.EventType == events.WalletDebitRequestedEvent
				})).Return(nil).Once()

				// Second publish fails
				publisher.EXPECT().Publish(mock.Anything, mock.Anything).
					Return(errors.New("publish error")).Once()
			},
			expectedError: "failed to publish payment events",
		},
		{
			name: "unsupported payment method",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				unsupportedPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(5000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodType("unsupported"),
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}

				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(unsupportedPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Twice() // Once for processing, once for failed

				// Expect payment events (processing and failed - variadic arguments)
				publisher.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "unsupported payment method - save failed payment error",
			command: &ProcessPaymentMethodCommand{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				unsupportedPayment := &domain.Payment{
					ID:     validPaymentID,
					UserID: validUserID,
					Amount: models.NewMoney(5000, "USD"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodType("unsupported"),
					},
					Status:     domain.PaymentStatusInitiated,
					Timestamps: models.NewTimestamps(),
				}

				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(unsupportedPayment, nil).Once()
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil).Once()  // First save succeeds
				repo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*domain.Payment")).
					Return(errors.New("save error")).Once() // Second save fails
			},
			expectedError: "failed to save failed payment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRepo := mocks.NewMockPaymentRepository(t)
			mockPublisher := mocks.NewMockPublisher(t)

			tt.setupMocks(mockRepo, mockPublisher)

			// Create use case
			useCase := NewProcessPaymentMethod(mockRepo, mockPublisher)

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