package application

import (
	"context"
	"testing"
	"time"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/payments-service/mocks"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetPayment_Execute(t *testing.T) {
	// Test data
	validPaymentID := "550e8400-e29b-41d4-a716-446655440020"
	validUserID := "550e8400-e29b-41d4-a716-446655440010"
	testTime := time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC)

	testPayment := &domain.Payment{
		ID:          models.ID(validPaymentID),
		UserID:      models.ID(validUserID),
		Amount:      models.NewMoney(5000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeWallet,
			WalletPaymentMethod: &domain.WalletPaymentMethod{
				WalletID: "550e8400-e29b-41d4-a716-446655440001",
			},
		},
		Description: "Test payment",
		Status:      domain.PaymentStatusCompleted,
		Timestamps: models.Timestamps{
			CreatedAt: testTime,
			UpdatedAt: testTime.Add(time.Minute),
		},
	}

	tests := []struct {
		name           string
		query          *GetPaymentQuery
		setupMocks     func(*mocks.MockPaymentRepository)
		expectedError  string
		expectedResult *GetPaymentResponse
	}{
		{
			name: "successful payment retrieval",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(testPayment, nil).Once()
			},
			expectedError: "",
			expectedResult: &GetPaymentResponse{
				PaymentID:     validPaymentID,
				UserID:        validUserID,
				Amount:        5000,
				Currency:      "USD",
				PaymentMethod: testPayment.PaymentMethod,
				Description:   "Test payment",
				Status:        "completed",
				CreatedAt:     testTime.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:     testTime.Add(time.Minute).Format("2006-01-02T15:04:05Z07:00"),
			},
		},
		{
			name: "empty payment ID",
			query: &GetPaymentQuery{
				PaymentID: "",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				// No expectations - should fail validation
			},
			expectedError:  "payment ID is required",
			expectedResult: nil,
		},
		{
			name: "invalid payment ID format",
			query: &GetPaymentQuery{
				PaymentID: "invalid-uuid",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				// No expectations - should fail validation
			},
			expectedError:  "invalid payment ID",
			expectedResult: nil,
		},
		{
			name: "payment not found - repository returns nil",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(nil, nil).Once()
			},
			expectedError:  "payment not found",
			expectedResult: nil,
		},
		{
			name: "repository error",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(nil, errors.New("database error")).Once()
			},
			expectedError:  "failed to find payment",
			expectedResult: nil,
		},
		{
			name: "payment with credit card method",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				paymentWithCard := &domain.Payment{
					ID:     models.ID(validPaymentID),
					UserID: models.ID(validUserID),
					Amount: models.NewMoney(10000, "EUR"),
					PaymentMethod: domain.PaymentMethod{
						PaymentMethodType: domain.PaymentMethodTypeCreditCard,
						CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
							CardToken: "tok_1234567890",
						},
					},
					Description: "Card payment",
					Status:      domain.PaymentStatusProcessing,
					Timestamps: models.Timestamps{
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(paymentWithCard, nil).Once()
			},
			expectedError: "",
			expectedResult: &GetPaymentResponse{
				PaymentID: validPaymentID,
				UserID:    validUserID,
				Amount:    10000,
				Currency:  "EUR",
				PaymentMethod: domain.PaymentMethod{
					PaymentMethodType: domain.PaymentMethodTypeCreditCard,
					CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
						CardToken: "tok_1234567890",
					},
				},
				Description: "Card payment",
				Status:      "processing",
				CreatedAt:   testTime.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:   testTime.Format("2006-01-02T15:04:05Z07:00"),
			},
		},
		{
			name: "payment with failed status",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				failedPayment := &domain.Payment{
					ID:            models.ID(validPaymentID),
					UserID:        models.ID(validUserID),
					Amount:        models.NewMoney(2500, "USD"),
					PaymentMethod: testPayment.PaymentMethod,
					Description:   "Failed payment",
					Status:        domain.PaymentStatusFailed,
					Timestamps: models.Timestamps{
						CreatedAt: testTime,
						UpdatedAt: testTime.Add(time.Hour),
					},
				}
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(failedPayment, nil).Once()
			},
			expectedError: "",
			expectedResult: &GetPaymentResponse{
				PaymentID:     validPaymentID,
				UserID:        validUserID,
				Amount:        2500,
				Currency:      "USD",
				PaymentMethod: testPayment.PaymentMethod,
				Description:   "Failed payment",
				Status:        "failed",
				CreatedAt:     testTime.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:     testTime.Add(time.Hour).Format("2006-01-02T15:04:05Z07:00"),
			},
		},
		{
			name: "payment with initiated status",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				initiatedPayment := &domain.Payment{
					ID:            models.ID(validPaymentID),
					UserID:        models.ID(validUserID),
					Amount:        models.NewMoney(7500, "GBP"),
					PaymentMethod: testPayment.PaymentMethod,
					Description:   "Initiated payment",
					Status:        domain.PaymentStatusInitiated,
					Timestamps: models.Timestamps{
						CreatedAt: testTime,
						UpdatedAt: testTime,
					},
				}
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(initiatedPayment, nil).Once()
			},
			expectedError: "",
			expectedResult: &GetPaymentResponse{
				PaymentID:     validPaymentID,
				UserID:        validUserID,
				Amount:        7500,
				Currency:      "GBP",
				PaymentMethod: testPayment.PaymentMethod,
				Description:   "Initiated payment",
				Status:        "initiated",
				CreatedAt:     testTime.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:     testTime.Format("2006-01-02T15:04:05Z07:00"),
			},
		},
		{
			name: "payment with cancelled status",
			query: &GetPaymentQuery{
				PaymentID: validPaymentID,
			},
			setupMocks: func(repo *mocks.MockPaymentRepository) {
				cancelledPayment := &domain.Payment{
					ID:            models.ID(validPaymentID),
					UserID:        models.ID(validUserID),
					Amount:        models.NewMoney(1000, "USD"),
					PaymentMethod: testPayment.PaymentMethod,
					Description:   "Cancelled payment",
					Status:        domain.PaymentStatusCancelled,
					Timestamps: models.Timestamps{
						CreatedAt: testTime,
						UpdatedAt: testTime.Add(30 * time.Minute),
					},
				}
				repo.EXPECT().FindByID(mock.Anything, models.ID(validPaymentID)).
					Return(cancelledPayment, nil).Once()
			},
			expectedError: "",
			expectedResult: &GetPaymentResponse{
				PaymentID:     validPaymentID,
				UserID:        validUserID,
				Amount:        1000,
				Currency:      "USD",
				PaymentMethod: testPayment.PaymentMethod,
				Description:   "Cancelled payment",
				Status:        "cancelled",
				CreatedAt:     testTime.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:     testTime.Add(30 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRepo := mocks.NewMockPaymentRepository(t)
			tt.setupMocks(mockRepo)

			// Create use case
			useCase := NewGetPayment(mockRepo)

			// Execute
			result, err := useCase.Execute(context.Background(), tt.query)

			// Assertions
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.PaymentID, result.PaymentID)
				assert.Equal(t, tt.expectedResult.UserID, result.UserID)
				assert.Equal(t, tt.expectedResult.Amount, result.Amount)
				assert.Equal(t, tt.expectedResult.Currency, result.Currency)
				assert.Equal(t, tt.expectedResult.Description, result.Description)
				assert.Equal(t, tt.expectedResult.Status, result.Status)
				assert.Equal(t, tt.expectedResult.CreatedAt, result.CreatedAt)
				assert.Equal(t, tt.expectedResult.UpdatedAt, result.UpdatedAt)

				// Deep compare payment method
				assert.Equal(t, tt.expectedResult.PaymentMethod.PaymentMethodType, result.PaymentMethod.PaymentMethodType)
				if tt.expectedResult.PaymentMethod.WalletPaymentMethod != nil {
					assert.NotNil(t, result.PaymentMethod.WalletPaymentMethod)
					assert.Equal(t, tt.expectedResult.PaymentMethod.WalletPaymentMethod.WalletID, result.PaymentMethod.WalletPaymentMethod.WalletID)
				}
				if tt.expectedResult.PaymentMethod.CreditCardPaymentMethod != nil {
					assert.NotNil(t, result.PaymentMethod.CreditCardPaymentMethod)
					assert.Equal(t, tt.expectedResult.PaymentMethod.CreditCardPaymentMethod.CardToken, result.PaymentMethod.CreditCardPaymentMethod.CardToken)
				}
			}
		})
	}
}