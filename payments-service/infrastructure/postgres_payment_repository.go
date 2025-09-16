package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"github.com/draftea/payment-system/payments-service/domain"
	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// PostgresPaymentRepository implements PaymentRepository using PostgreSQL
type PostgresPaymentRepository struct {
	db *sqlx.DB
}

// NewPostgresPaymentRepository creates a new PostgresPaymentRepository
func NewPostgresPaymentRepository(db *sqlx.DB) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{db: db}
}

// postgresPayment represents payment in database
type postgresPayment struct {
	ID                  string     `db:"id"`
	UserID              string     `db:"user_id"`
	Amount              int64      `db:"amount"`
	Currency            string     `db:"currency"`
	PaymentMethodType   string     `db:"payment_method_type"`
	PaymentMethodWallet *string    `db:"payment_method_wallet_id"`
	Description         string     `db:"description"`
	Status              string     `db:"status"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"`
	DeletedAt           *time.Time `db:"deleted_at"`
	Version             int        `db:"version"`
}

// Save saves a payment to the database
func (r *PostgresPaymentRepository) Save(ctx context.Context, payment *domain.Payment) error {
	// Process events to determine operation type
	for _, event := range payment.Events() {
		switch event.EventType {
		case events.PaymentCreatedEvent:
			return r.insertPayment(ctx, payment)
		case events.PaymentProcessingEvent, events.PaymentCompletedEvent,
			events.PaymentFailedEvent, events.PaymentCancelledEvent:
			return r.updatePayment(ctx, payment)
		}
	}
	return nil
}

// insertPayment inserts a new payment
func (r *PostgresPaymentRepository) insertPayment(ctx context.Context, payment *domain.Payment) error {
	query := `
		INSERT INTO payments (
			id, user_id, amount, currency, payment_method_type,
			payment_method_wallet_id, description, status,
			created_at, updated_at, version
		) VALUES (
			:id, :user_id, :amount, :currency, :payment_method_type,
			:payment_method_wallet_id, :description, :status,
			:created_at, :updated_at, :version
		)`

	pgPayment := r.toPostgres(payment)
	_, err := r.db.NamedExecContext(ctx, query, pgPayment)
	if err != nil {
		return errors.Wrap(err, "failed to insert payment")
	}

	return nil
}

// updatePayment updates an existing payment
func (r *PostgresPaymentRepository) updatePayment(ctx context.Context, payment *domain.Payment) error {
	query := `
		UPDATE payments
		SET status = :status, updated_at = :updated_at, version = :version
		WHERE id = :id AND version = :old_version`

	_, err := r.db.NamedExecContext(ctx, query, map[string]interface{}{
		"id":          payment.ID.String(),
		"status":      string(payment.Status),
		"updated_at":  payment.Timestamps.UpdatedAt,
		"version":     payment.Version.Value,
		"old_version": payment.Version.Value - 1, // Optimistic locking
	})

	if err != nil {
		return errors.Wrap(err, "failed to update payment")
	}

	return nil
}

// FindByID finds a payment by ID
func (r *PostgresPaymentRepository) FindByID(ctx context.Context, id models.ID) (*domain.Payment, error) {
	query := `
		SELECT id, user_id, amount, currency, payment_method_type,
			   payment_method_wallet_id, description, status,
			   created_at, updated_at, deleted_at, version
		FROM payments
		WHERE id = $1 AND deleted_at IS NULL`

	var pgPayment postgresPayment
	err := r.db.GetContext(ctx, &pgPayment, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Payment not found
		}
		return nil, errors.Wrap(err, "failed to find payment")
	}

	return r.toDomain(&pgPayment)
}

// FindByUserID finds payments by user ID
func (r *PostgresPaymentRepository) FindByUserID(ctx context.Context, userID models.ID) ([]*domain.Payment, error) {
	query := `
		SELECT id, user_id, amount, currency, payment_method_type,
			   payment_method_wallet_id, description, status,
			   created_at, updated_at, deleted_at, version
		FROM payments
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	var pgPayments []postgresPayment
	err := r.db.SelectContext(ctx, &pgPayments, query, userID.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to find payments by user ID")
	}

	payments := make([]*domain.Payment, len(pgPayments))
	for i, pgPayment := range pgPayments {
		payment, err := r.toDomain(&pgPayment)
		if err != nil {
			return nil, err
		}
		payments[i] = payment
	}

	return payments, nil
}

// toPostgres converts domain payment to postgres model
func (r *PostgresPaymentRepository) toPostgres(payment *domain.Payment) *postgresPayment {
	var walletID *string
	if payment.PaymentMethod.WalletPaymentMethod != nil && payment.PaymentMethod.WalletPaymentMethod.WalletID != "" {
		walletID = &payment.PaymentMethod.WalletPaymentMethod.WalletID
	}

	return &postgresPayment{
		ID:                  payment.ID.String(),
		UserID:              payment.UserID.String(),
		Amount:              payment.Amount.Amount,
		Currency:            payment.Amount.Currency,
		PaymentMethodType:   payment.PaymentMethod.PaymentMethodType.String(),
		PaymentMethodWallet: walletID,
		Description:         payment.Description,
		Status:              string(payment.Status),
		CreatedAt:           payment.Timestamps.CreatedAt,
		UpdatedAt:           payment.Timestamps.UpdatedAt,
		DeletedAt:           payment.Timestamps.DeletedAt,
		Version:             payment.Version.Value,
	}
}

// toDomain converts postgres model to domain payment
func (r *PostgresPaymentRepository) toDomain(pgPayment *postgresPayment) (*domain.Payment, error) {
	id, err := models.NewID(pgPayment.ID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid payment ID")
	}

	userID, err := models.NewID(pgPayment.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID")
	}

	amount := models.NewMoney(pgPayment.Amount, pgPayment.Currency)

	paymentMethodType, err := domain.NewPaymentMethodType(pgPayment.PaymentMethodType)
	if err != nil {
		return nil, errors.Wrap(err, "invalid payment method type")
	}

	var creator *domain.PaymentMethodCreator
	if pgPayment.PaymentMethodWallet != nil {
		creator = &domain.PaymentMethodCreator{
			WalletID: pgPayment.PaymentMethodWallet,
		}
	} else {
		creator = &domain.PaymentMethodCreator{}
	}

	paymentMethod, err := domain.NewPaymentMethod(*paymentMethodType, creator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create payment method")
	}

	payment := &domain.Payment{
		ID:            id,
		UserID:        userID,
		Amount:        amount,
		PaymentMethod: *paymentMethod,
		Description:   pgPayment.Description,
		Status:        domain.PaymentStatus(pgPayment.Status),
		Timestamps: models.Timestamps{
			CreatedAt: pgPayment.CreatedAt,
			UpdatedAt: pgPayment.UpdatedAt,
			DeletedAt: pgPayment.DeletedAt,
		},
		Version: models.Version{Value: pgPayment.Version},
	}

	return payment, nil
}
