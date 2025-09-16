package application

import (
	"context"
	"testing"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/payments-service/mocks"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessWalletDebit_Execute(t *testing.T) {
	validPaymentID := models.ID("550e8400-e29b-41d4-a716-446655440020")
	validUserID := models.ID("550e8400-e29b-41d4-a716-446655440010")
	walletID := "550e8400-e29b-41d4-a716-446655440001"

	walletPayment := &domain.Payment{
		ID:     validPaymentID,
		UserID: validUserID,
		Amount: models.NewMoney(5000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeWallet,
			WalletPaymentMethod: &domain.WalletPaymentMethod{
				WalletID: walletID,
			},
		},
		Status:     domain.PaymentStatusProcessing,
		Timestamps: models.NewTimestamps(),
	}

	creditCardPayment := &domain.Payment{
		ID:     validPaymentID,
		UserID: validUserID,
		Amount: models.NewMoney(5000, "USD"),
		PaymentMethod: domain.PaymentMethod{
			PaymentMethodType: domain.PaymentMethodTypeCreditCard,
			CreditCardPaymentMethod: &domain.CreditCardPaymentMethod{
				CardToken: "tok_1234567890",
			},
		},
		Status:     domain.PaymentStatusProcessing,
		Timestamps: models.NewTimestamps(),
	}

	tests := []struct {
		name          string
		command       *ProcessWalletDebitCommand
		setupMocks    func(*mocks.MockPaymentRepository, *mocks.MockPublisher)
		expectedError string
	}{
		{
			name: "successful wallet debit processing - completed",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_completed_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(walletPayment, nil).Once()

				publisher.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "successful wallet debit processing - failed",
			command: &ProcessWalletDebitCommand{
				PaymentID:    validPaymentID,
				WalletID:     walletID,
				Amount:       models.NewMoney(5000, "USD"),
				Status:       "failed",
				ErrorCode:    "insufficient_funds",
				ErrorMessage: "Insufficient funds in wallet",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(walletPayment, nil).Once()

				publisher.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectedError: "",
		},
		{
			name: "payment not found",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(nil, nil).Once()
			},
			expectedError: "payment not found",
		},
		{
			name: "repository find error",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).
					Return(nil, errors.New("database error")).Once()
			},
			expectedError: "failed to find payment",
		},
		{
			name: "not a wallet payment",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(creditCardPayment, nil).Once()
			},
			expectedError: "payment is not a wallet payment",
		},
		{
			name: "publisher error",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				repo.EXPECT().FindByID(mock.Anything, validPaymentID).Return(walletPayment, nil).Once()
				publisher.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("publisher error")).Once()
			},
			expectedError: "failed to publish payment operation events",
		},
		{
			name: "validation error - empty payment ID",
			command: &ProcessWalletDebitCommand{
				PaymentID:     models.ID(""),
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "payment ID is required",
		},
		{
			name: "validation error - empty wallet ID",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "wallet ID is required",
		},
		{
			name: "validation error - zero amount",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(0, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "amount must be positive",
		},
		{
			name: "validation error - empty status",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "status is required",
		},
		{
			name: "validation error - invalid status",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "pending",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "status must be either 'completed' or 'failed'",
		},
		{
			name: "validation error - completed without transaction ID",
			command: &ProcessWalletDebitCommand{
				PaymentID: validPaymentID,
				WalletID:  walletID,
				Amount:    models.NewMoney(5000, "USD"),
				Status:    "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "transaction ID is required for completed operations",
		},
		{
			name: "validation error - failed without error code",
			command: &ProcessWalletDebitCommand{
				PaymentID:    validPaymentID,
				WalletID:     walletID,
				Amount:       models.NewMoney(5000, "USD"),
				Status:       "failed",
				ErrorMessage: "Some error",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "error code is required for failed operations",
		},
		{
			name: "negative amount validation",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      walletID,
				TransactionID: "txn_123",
				Amount:        models.NewMoney(-1000, "USD"),
				Status:        "completed",
			},
			setupMocks: func(repo *mocks.MockPaymentRepository, publisher *mocks.MockPublisher) {
				// No expectations - should fail validation
			},
			expectedError: "amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockRepo := mocks.NewMockPaymentRepository(t)
			mockPublisher := mocks.NewMockPublisher(t)

			tt.setupMocks(mockRepo, mockPublisher)

			// Create use case
			useCase := NewProcessWalletDebit(mockRepo, mockPublisher)

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

func TestProcessWalletDebit_validateCommand(t *testing.T) {
	useCase := &ProcessWalletDebit{}
	validPaymentID := models.ID("550e8400-e29b-41d4-a716-446655440020")

	tests := []struct {
		name          string
		command       *ProcessWalletDebitCommand
		expectedError string
	}{
		{
			name: "valid completed command",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			expectedError: "",
		},
		{
			name: "valid failed command",
			command: &ProcessWalletDebitCommand{
				PaymentID:    validPaymentID,
				WalletID:     "wallet_123",
				Amount:       models.NewMoney(5000, "USD"),
				Status:       "failed",
				ErrorCode:    "insufficient_funds",
				ErrorMessage: "Insufficient funds",
			},
			expectedError: "",
		},
		{
			name: "empty payment ID",
			command: &ProcessWalletDebitCommand{
				PaymentID:     models.ID(""),
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			expectedError: "payment ID is required",
		},
		{
			name: "empty wallet ID",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "completed",
			},
			expectedError: "wallet ID is required",
		},
		{
			name: "zero amount",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(0, "USD"),
				Status:        "completed",
			},
			expectedError: "amount must be positive",
		},
		{
			name: "negative amount",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(-1000, "USD"),
				Status:        "completed",
			},
			expectedError: "amount must be positive",
		},
		{
			name: "empty status",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "",
			},
			expectedError: "status is required",
		},
		{
			name: "invalid status",
			command: &ProcessWalletDebitCommand{
				PaymentID:     validPaymentID,
				WalletID:      "wallet_123",
				TransactionID: "txn_123",
				Amount:        models.NewMoney(5000, "USD"),
				Status:        "invalid_status",
			},
			expectedError: "status must be either 'completed' or 'failed'",
		},
		{
			name: "completed without transaction ID",
			command: &ProcessWalletDebitCommand{
				PaymentID: validPaymentID,
				WalletID:  "wallet_123",
				Amount:    models.NewMoney(5000, "USD"),
				Status:    "completed",
			},
			expectedError: "transaction ID is required for completed operations",
		},
		{
			name: "failed without error code",
			command: &ProcessWalletDebitCommand{
				PaymentID:    validPaymentID,
				WalletID:     "wallet_123",
				Amount:       models.NewMoney(5000, "USD"),
				Status:       "failed",
				ErrorMessage: "Some error",
			},
			expectedError: "error code is required for failed operations",
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