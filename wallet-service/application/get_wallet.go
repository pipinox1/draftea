package application

import (
	"context"
	"time"

	"github.com/draftea/payment-system/shared/models"
	"github.com/draftea/payment-system/wallet-service/domain"
	"github.com/draftea/payment-system/shared/telemetry"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GetWalletQuery represents the query to get a wallet
type GetWalletQuery struct {
	WalletID string `json:"wallet_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// GetWalletResponse represents the response for getting a wallet
type GetWalletResponse struct {
	WalletID  string       `json:"wallet_id"`
	UserID    string       `json:"user_id"`
	Balance   models.Money `json:"balance"`
	Status    string       `json:"status"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
}

// GetWallet use case
type GetWallet struct {
	walletRepository domain.WalletRepository
}

// NewGetWallet creates a new GetWallet use case
func NewGetWallet(walletRepository domain.WalletRepository) *GetWallet {
	return &GetWallet{
		walletRepository: walletRepository,
	}
}

// Execute executes the get wallet use case
func (uc *GetWallet) Execute(ctx context.Context, query *GetWalletQuery) (*GetWalletResponse, error) {
	// Start tracing span
	start := time.Now()
	ctx, span := telemetry.StartSpan(ctx, "get_wallet",
		trace.WithAttributes(
			attribute.String("wallet_id", query.WalletID),
			attribute.String("user_id", query.UserID),
		),
	)
	defer span.End()

	var status string = "error"
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCounter(ctx, "wallet_operations_total", "Total wallet operations", 1,
			attribute.String("operation", "get_wallet"),
			attribute.String("status", status),
		)
		telemetry.RecordHistogram(ctx, "wallet_operation_duration_seconds", "Wallet operation duration", duration.Seconds(),
			attribute.String("operation", "get_wallet"),
			attribute.String("status", status),
		)
	}()

	var wallet *domain.Wallet
	var err error

	if query.WalletID != "" {
		walletID, parseErr := models.NewID(query.WalletID)
		if parseErr != nil {
			status = "error"
			span.RecordError(parseErr)
			return nil, errors.Wrap(parseErr, "invalid wallet ID")
		}

		wallet, err = uc.walletRepository.FindByID(ctx, walletID)
		if err != nil {
			status = "error"
			span.RecordError(err)
			return nil, errors.Wrap(err, "failed to find wallet by ID")
		}
	} else if query.UserID != "" {
		userID, err := models.NewID(query.UserID)
		if err != nil {
			status = "error"
			span.RecordError(err)
			return nil, errors.Wrap(err, "invalid user ID")
		}

		wallet, err = uc.walletRepository.FindByUserID(ctx, userID)
		if err != nil {
			status = "error"
			span.RecordError(err)
			return nil, errors.Wrap(err, "failed to find wallet by user ID")
		}
	} else {
		status = "error"
		err := errors.New("either wallet_id or user_id is required")
		span.RecordError(err)
		return nil, err
	}

	if wallet == nil {
		status = "error"
		err := errors.New("wallet not found")
		span.RecordError(err)
		return nil, err
	}

	// Add wallet attributes to span
	span.SetAttributes(
		attribute.String("found_wallet_id", wallet.ID.String()),
		attribute.String("found_user_id", wallet.UserID.String()),
		attribute.Float64("balance", float64(wallet.Balance.Amount)/100.0),
		attribute.String("wallet_status", string(wallet.Status)),
	)

	// Record wallet balance metric
	telemetry.RecordGauge(ctx, "wallet_balance", "Current wallet balance", float64(wallet.Balance.Amount)/100.0,
		attribute.String("wallet_id", wallet.ID.String()),
	)

	// Convert to response
	response := &GetWalletResponse{
		WalletID:  wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Balance:   wallet.Balance,
		Status:    string(wallet.Status),
		CreatedAt: wallet.Timestamps.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: wallet.Timestamps.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	status = "success"
	return response, nil
}