package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/draftea/payment-system/wallet-service/domain"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// PostgresWalletRepository implements WalletRepository using PostgreSQL
type PostgresWalletRepository struct {
	db *sqlx.DB
}

// NewPostgresWalletRepository creates a new PostgresWalletRepository
func NewPostgresWalletRepository(db *sqlx.DB) *PostgresWalletRepository {
	return &PostgresWalletRepository{db: db}
}

// postgresWallet represents wallet in database
type postgresWallet struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	Balance   int64      `db:"balance"`
	Currency  string     `db:"currency"`
	Status    string     `db:"status"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
	Version   int        `db:"version"`
}

// Save saves a wallet to the database
func (r *PostgresWalletRepository) Save(ctx context.Context, wallet *domain.Wallet) error {
	// Process events to determine operation type
	for _, event := range wallet.Events() {
		switch event.EventType {
		case "wallet.created":
			return r.insertWallet(ctx, wallet)
		case events.WalletDebitedEvent, events.WalletCreditedEvent,
			 events.WalletFrozenEvent, events.WalletUnfrozenEvent:
			return r.updateWallet(ctx, wallet)
		}
	}
	return nil
}

// insertWallet inserts a new wallet
func (r *PostgresWalletRepository) insertWallet(ctx context.Context, wallet *domain.Wallet) error {
	query := `
		INSERT INTO wallets (
			id, user_id, balance, currency, status,
			created_at, updated_at, version
		) VALUES (
			:id, :user_id, :balance, :currency, :status,
			:created_at, :updated_at, :version
		)`

	pgWallet := r.toPostgres(wallet)
	_, err := r.db.NamedExecContext(ctx, query, pgWallet)
	if err != nil {
		return errors.Wrap(err, "failed to insert wallet")
	}

	return nil
}

// updateWallet updates an existing wallet
func (r *PostgresWalletRepository) updateWallet(ctx context.Context, wallet *domain.Wallet) error {
	query := `
		UPDATE wallets
		SET balance = :balance, status = :status, updated_at = :updated_at, version = :version
		WHERE id = :id AND version = :old_version`

	_, err := r.db.NamedExecContext(ctx, query, map[string]interface{}{
		"id":          wallet.ID.String(),
		"balance":     wallet.Balance.Amount,
		"status":      string(wallet.Status),
		"updated_at":  wallet.Timestamps.UpdatedAt,
		"version":     wallet.Version.Value,
		"old_version": wallet.Version.Value - 1, // Optimistic locking
	})

	if err != nil {
		return errors.Wrap(err, "failed to update wallet")
	}

	return nil
}

// FindByID finds a wallet by ID
func (r *PostgresWalletRepository) FindByID(ctx context.Context, id models.ID) (*domain.Wallet, error) {
	query := `
		SELECT id, user_id, balance, currency, status,
			   created_at, updated_at, deleted_at, version
		FROM wallets
		WHERE id = $1 AND deleted_at IS NULL`

	var pgWallet postgresWallet
	err := r.db.GetContext(ctx, &pgWallet, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Wallet not found
		}
		return nil, errors.Wrap(err, "failed to find wallet")
	}

	return r.toDomain(&pgWallet)
}

// FindByUserID finds a wallet by user ID
func (r *PostgresWalletRepository) FindByUserID(ctx context.Context, userID models.ID) (*domain.Wallet, error) {
	query := `
		SELECT id, user_id, balance, currency, status,
			   created_at, updated_at, deleted_at, version
		FROM wallets
		WHERE user_id = $1 AND deleted_at IS NULL
		LIMIT 1`

	var pgWallet postgresWallet
	err := r.db.GetContext(ctx, &pgWallet, query, userID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Wallet not found
		}
		return nil, errors.Wrap(err, "failed to find wallet by user ID")
	}

	return r.toDomain(&pgWallet)
}

// toPostgres converts domain wallet to postgres model
func (r *PostgresWalletRepository) toPostgres(wallet *domain.Wallet) *postgresWallet {
	return &postgresWallet{
		ID:        wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Balance:   wallet.Balance.Amount,
		Currency:  wallet.Balance.Currency,
		Status:    string(wallet.Status),
		CreatedAt: wallet.Timestamps.CreatedAt,
		UpdatedAt: wallet.Timestamps.UpdatedAt,
		DeletedAt: wallet.Timestamps.DeletedAt,
		Version:   wallet.Version.Value,
	}
}

// toDomain converts postgres model to domain wallet
func (r *PostgresWalletRepository) toDomain(pgWallet *postgresWallet) (*domain.Wallet, error) {
	id, err := models.NewID(pgWallet.ID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid wallet ID")
	}

	userID, err := models.NewID(pgWallet.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid user ID")
	}

	balance := models.NewMoney(pgWallet.Balance, pgWallet.Currency)

	wallet := &domain.Wallet{
		ID:      id,
		UserID:  userID,
		Balance: balance,
		Status:  domain.WalletStatus(pgWallet.Status),
		Timestamps: models.Timestamps{
			CreatedAt: pgWallet.CreatedAt,
			UpdatedAt: pgWallet.UpdatedAt,
			DeletedAt: pgWallet.DeletedAt,
		},
		Version: models.Version{Value: pgWallet.Version},
	}

	return wallet, nil
}

// PostgresTransactionRepository implements TransactionRepository using PostgreSQL
type PostgresTransactionRepository struct {
	db *sqlx.DB
}

// NewPostgresTransactionRepository creates a new PostgresTransactionRepository
func NewPostgresTransactionRepository(db *sqlx.DB) *PostgresTransactionRepository {
	return &PostgresTransactionRepository{db: db}
}

// postgresTransaction represents transaction in database
type postgresTransaction struct {
	ID            string     `db:"id"`
	WalletID      string     `db:"wallet_id"`
	Type          string     `db:"type"`
	Amount        int64      `db:"amount"`
	Currency      string     `db:"currency"`
	BalanceBefore int64      `db:"balance_before"`
	BalanceAfter  int64      `db:"balance_after"`
	Reference     string     `db:"reference"`
	PaymentID     *string    `db:"payment_id"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

// Save saves a transaction to the database
func (r *PostgresTransactionRepository) Save(ctx context.Context, transaction *domain.Transaction) error {
	query := `
		INSERT INTO wallet_transactions (
			id, wallet_id, type, amount, currency, balance_before,
			balance_after, reference, payment_id, created_at, updated_at
		) VALUES (
			:id, :wallet_id, :type, :amount, :currency, :balance_before,
			:balance_after, :reference, :payment_id, :created_at, :updated_at
		)`

	pgTransaction := r.transactionToPostgres(transaction)
	_, err := r.db.NamedExecContext(ctx, query, pgTransaction)
	if err != nil {
		return errors.Wrap(err, "failed to insert transaction")
	}

	return nil
}

// FindByID finds a transaction by ID
func (r *PostgresTransactionRepository) FindByID(ctx context.Context, id models.ID) (*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, type, amount, currency, balance_before,
			   balance_after, reference, payment_id, created_at, updated_at, deleted_at
		FROM wallet_transactions
		WHERE id = $1 AND deleted_at IS NULL`

	var pgTransaction postgresTransaction
	err := r.db.GetContext(ctx, &pgTransaction, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Transaction not found
		}
		return nil, errors.Wrap(err, "failed to find transaction")
	}

	return r.transactionToDomain(&pgTransaction)
}

// FindByWalletID finds transactions by wallet ID
func (r *PostgresTransactionRepository) FindByWalletID(ctx context.Context, walletID models.ID, limit, offset int) ([]*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, type, amount, currency, balance_before,
			   balance_after, reference, payment_id, created_at, updated_at, deleted_at
		FROM wallet_transactions
		WHERE wallet_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	var pgTransactions []postgresTransaction
	err := r.db.SelectContext(ctx, &pgTransactions, query, walletID.String(), limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find transactions by wallet ID")
	}

	transactions := make([]*domain.Transaction, len(pgTransactions))
	for i, pgTx := range pgTransactions {
		transaction, err := r.transactionToDomain(&pgTx)
		if err != nil {
			return nil, err
		}
		transactions[i] = transaction
	}

	return transactions, nil
}

// FindByPaymentID finds transactions by payment ID
func (r *PostgresTransactionRepository) FindByPaymentID(ctx context.Context, paymentID models.ID) ([]*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, type, amount, currency, balance_before,
			   balance_after, reference, payment_id, created_at, updated_at, deleted_at
		FROM wallet_transactions
		WHERE payment_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	var pgTransactions []postgresTransaction
	err := r.db.SelectContext(ctx, &pgTransactions, query, paymentID.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to find transactions by payment ID")
	}

	transactions := make([]*domain.Transaction, len(pgTransactions))
	for i, pgTx := range pgTransactions {
		transaction, err := r.transactionToDomain(&pgTx)
		if err != nil {
			return nil, err
		}
		transactions[i] = transaction
	}

	return transactions, nil
}

// transactionToPostgres converts domain transaction to postgres model
func (r *PostgresTransactionRepository) transactionToPostgres(transaction *domain.Transaction) *postgresTransaction {
	var paymentID *string
	if transaction.PaymentID != nil {
		pid := transaction.PaymentID.String()
		paymentID = &pid
	}

	return &postgresTransaction{
		ID:            transaction.ID.String(),
		WalletID:      transaction.WalletID.String(),
		Type:          string(transaction.Type),
		Amount:        transaction.Amount.Amount,
		Currency:      transaction.Amount.Currency,
		BalanceBefore: transaction.BalanceBefore.Amount,
		BalanceAfter:  transaction.BalanceAfter.Amount,
		Reference:     transaction.Reference,
		PaymentID:     paymentID,
		CreatedAt:     transaction.Timestamps.CreatedAt,
		UpdatedAt:     transaction.Timestamps.UpdatedAt,
		DeletedAt:     transaction.Timestamps.DeletedAt,
	}
}

// transactionToDomain converts postgres model to domain transaction
func (r *PostgresTransactionRepository) transactionToDomain(pgTx *postgresTransaction) (*domain.Transaction, error) {
	id, err := models.NewID(pgTx.ID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid transaction ID")
	}

	walletID, err := models.NewID(pgTx.WalletID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid wallet ID")
	}

	var paymentID *models.ID
	if pgTx.PaymentID != nil {
		pid, err := models.NewID(*pgTx.PaymentID)
		if err != nil {
			return nil, errors.Wrap(err, "invalid payment ID")
		}
		paymentID = &pid
	}

	amount := models.NewMoney(pgTx.Amount, pgTx.Currency)
	balanceBefore := models.NewMoney(pgTx.BalanceBefore, pgTx.Currency)
	balanceAfter := models.NewMoney(pgTx.BalanceAfter, pgTx.Currency)

	transaction := &domain.Transaction{
		ID:            id,
		WalletID:      walletID,
		Type:          domain.TransactionType(pgTx.Type),
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Reference:     pgTx.Reference,
		PaymentID:     paymentID,
		Timestamps: models.Timestamps{
			CreatedAt: pgTx.CreatedAt,
			UpdatedAt: pgTx.UpdatedAt,
			DeletedAt: pgTx.DeletedAt,
		},
	}

	return transaction, nil
}