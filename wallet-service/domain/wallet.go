package domain

import (
	"context"

	"github.com/draftea/payment-system/shared/events"
	"github.com/draftea/payment-system/shared/models"
	"github.com/pkg/errors"
)

// WalletStatus represents the status of a wallet
type WalletStatus string

const (
	WalletStatusActive  WalletStatus = "active"
	WalletStatusFrozen  WalletStatus = "frozen"
	WalletStatusClosed  WalletStatus = "closed"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeDebit      TransactionType = "debit"
	TransactionTypeCredit     TransactionType = "credit"
	TransactionTypeRefund     TransactionType = "refund"
	TransactionTypeAdjustment TransactionType = "adjustment"
)

// MovementType represents the type of movement as per documentation
type MovementType string

const (
	MovementTypeIncome  MovementType = "income"
	MovementTypeExpense MovementType = "expense"
)

// Wallet aggregate root
type Wallet struct {
	ID         models.ID      `json:"id"`
	UserID     models.ID      `json:"user_id"`
	Balance    models.Money   `json:"balance"`
	Status     WalletStatus   `json:"status"`
	Timestamps models.Timestamps
	Version    models.Version

	events []*events.Event
}

// Transaction represents a wallet transaction
type Transaction struct {
	ID            models.ID       `json:"id"`
	WalletID      models.ID       `json:"wallet_id"`
	Type          TransactionType `json:"type"`
	Amount        models.Money    `json:"amount"`
	BalanceBefore models.Money    `json:"balance_before"`
	BalanceAfter  models.Money    `json:"balance_after"`
	Reference     string          `json:"reference"`
	PaymentID     *models.ID      `json:"payment_id,omitempty"`
	Timestamps    models.Timestamps
}

// Movement represents a wallet movement as per documentation
type Movement struct {
	ID        models.ID    `json:"id"`
	Type      MovementType `json:"type"`
	Amount    int64        `json:"amount"`
	Currency  string       `json:"currency"`
	WalletID  models.ID    `json:"wallet_id"`
	Timestamps models.Timestamps
}

// CreateWallet factory method
func CreateWallet(userID models.ID, currency string) (*Wallet, error) {
	wallet := &Wallet{
		ID:         models.GenerateUUID(),
		UserID:     userID,
		Balance:    models.NewMoney(0, currency),
		Status:     WalletStatusActive,
		Timestamps: models.NewTimestamps(),
		Version:    models.NewVersion(),
	}

	// Record domain event
	event := events.NewEvent(wallet.ID, "wallet.created", WalletCreatedData{
		WalletID: wallet.ID,
		UserID:   wallet.UserID,
		Currency: wallet.Balance.Currency,
	})

	wallet.recordEvent(event)
	return wallet, nil
}

// Debit debits amount from wallet
func (w *Wallet) Debit(amount models.Money, paymentID models.ID, reference string) (*Transaction, error) {
	if w.Status != WalletStatusActive {
		return nil, errors.New("wallet is not active")
	}

	if amount.Currency != w.Balance.Currency {
		return nil, errors.New("currency mismatch")
	}

	if !amount.IsPositive() {
		return nil, errors.New("debit amount must be positive")
	}

	if w.Balance.Amount < amount.Amount {
		// Record insufficient funds event
		event := events.NewEvent(w.ID, events.InsufficientFundsEvent, InsufficientFundsData{
			WalletID:        w.ID,
			UserID:          w.UserID,
			PaymentID:       paymentID,
			RequestedAmount: amount,
			AvailableBalance: w.Balance,
			Shortfall:       models.NewMoney(amount.Amount-w.Balance.Amount, amount.Currency),
		})
		w.recordEvent(event)
		return nil, errors.New("insufficient funds")
	}

	// Create transaction
	transaction := &Transaction{
		ID:            models.GenerateUUID(),
		WalletID:      w.ID,
		Type:          TransactionTypeDebit,
		Amount:        amount,
		BalanceBefore: w.Balance,
		Reference:     reference,
		PaymentID:     &paymentID,
		Timestamps:    models.NewTimestamps(),
	}

	// Update balance
	newBalance, _ := w.Balance.Subtract(amount)
	w.Balance = newBalance
	transaction.BalanceAfter = w.Balance

	// Update wallet
	w.Timestamps = w.Timestamps.Update()
	w.Version = w.Version.Update()

	// Record events
	debitEvent := events.NewEvent(w.ID, events.WalletDebitedEvent, WalletDebitedData{
		WalletID:      w.ID,
		UserID:        w.UserID,
		PaymentID:     paymentID,
		TransactionID: transaction.ID,
		Amount:        amount,
		BalanceBefore: transaction.BalanceBefore,
		BalanceAfter:  transaction.BalanceAfter,
		Reference:     reference,
	})

	w.recordEvent(debitEvent)
	return transaction, nil
}

// Credit credits amount to wallet
func (w *Wallet) Credit(amount models.Money, reference string, paymentID *models.ID) (*Transaction, error) {
	if w.Status == WalletStatusClosed {
		return nil, errors.New("wallet is closed")
	}

	if amount.Currency != w.Balance.Currency {
		return nil, errors.New("currency mismatch")
	}

	if !amount.IsPositive() {
		return nil, errors.New("credit amount must be positive")
	}

	// Create transaction
	transaction := &Transaction{
		ID:            models.GenerateUUID(),
		WalletID:      w.ID,
		Type:          TransactionTypeCredit,
		Amount:        amount,
		BalanceBefore: w.Balance,
		Reference:     reference,
		PaymentID:     paymentID,
		Timestamps:    models.NewTimestamps(),
	}

	// Update balance
	newBalance, _ := w.Balance.Add(amount)
	w.Balance = newBalance
	transaction.BalanceAfter = w.Balance

	// Update wallet
	w.Timestamps = w.Timestamps.Update()
	w.Version = w.Version.Update()

	// Record event
	creditEvent := events.NewEvent(w.ID, events.WalletCreditedEvent, WalletCreditedData{
		WalletID:      w.ID,
		UserID:        w.UserID,
		TransactionID: transaction.ID,
		Amount:        amount,
		BalanceBefore: transaction.BalanceBefore,
		BalanceAfter:  transaction.BalanceAfter,
		Reference:     reference,
	})

	if paymentID != nil {
		creditEvent.WithMetadata("payment_id", paymentID.String())
	}

	w.recordEvent(creditEvent)
	return transaction, nil
}

// Freeze freezes the wallet
func (w *Wallet) Freeze() error {
	if w.Status == WalletStatusClosed {
		return errors.New("cannot freeze a closed wallet")
	}

	w.Status = WalletStatusFrozen
	w.Timestamps = w.Timestamps.Update()
	w.Version = w.Version.Update()

	event := events.NewEvent(w.ID, events.WalletFrozenEvent, WalletFrozenData{
		WalletID: w.ID,
		UserID:   w.UserID,
	})

	w.recordEvent(event)
	return nil
}

// Unfreeze unfreezes the wallet
func (w *Wallet) Unfreeze() error {
	if w.Status != WalletStatusFrozen {
		return errors.New("wallet is not frozen")
	}

	w.Status = WalletStatusActive
	w.Timestamps = w.Timestamps.Update()
	w.Version = w.Version.Update()

	event := events.NewEvent(w.ID, events.WalletUnfrozenEvent, WalletUnfrozenData{
		WalletID: w.ID,
		UserID:   w.UserID,
	})

	w.recordEvent(event)
	return nil
}

// CanDebit checks if wallet can debit the specified amount
func (w *Wallet) CanDebit(amount models.Money) bool {
	return w.Status == WalletStatusActive &&
		   w.Balance.Currency == amount.Currency &&
		   w.Balance.Amount >= amount.Amount
}

// Events returns domain events
func (w *Wallet) Events() []*events.Event {
	return w.events
}

// ClearEvents clears domain events
func (w *Wallet) ClearEvents() {
	w.events = make([]*events.Event, 0)
}

// recordEvent records a domain event
func (w *Wallet) recordEvent(event *events.Event) {
	w.events = append(w.events, event)
}

// Event Data Structures
type WalletCreatedData struct {
	WalletID models.ID `json:"wallet_id"`
	UserID   models.ID `json:"user_id"`
	Currency string    `json:"currency"`
}

type WalletDebitedData struct {
	WalletID      models.ID    `json:"wallet_id"`
	UserID        models.ID    `json:"user_id"`
	PaymentID     models.ID    `json:"payment_id"`
	TransactionID models.ID    `json:"transaction_id"`
	Amount        models.Money `json:"amount"`
	BalanceBefore models.Money `json:"balance_before"`
	BalanceAfter  models.Money `json:"balance_after"`
	Reference     string       `json:"reference"`
}

type WalletCreditedData struct {
	WalletID      models.ID    `json:"wallet_id"`
	UserID        models.ID    `json:"user_id"`
	TransactionID models.ID    `json:"transaction_id"`
	Amount        models.Money `json:"amount"`
	BalanceBefore models.Money `json:"balance_before"`
	BalanceAfter  models.Money `json:"balance_after"`
	Reference     string       `json:"reference"`
}

type InsufficientFundsData struct {
	WalletID         models.ID    `json:"wallet_id"`
	UserID           models.ID    `json:"user_id"`
	PaymentID        models.ID    `json:"payment_id"`
	RequestedAmount  models.Money `json:"requested_amount"`
	AvailableBalance models.Money `json:"available_balance"`
	Shortfall        models.Money `json:"shortfall"`
}

type WalletFrozenData struct {
	WalletID models.ID `json:"wallet_id"`
	UserID   models.ID `json:"user_id"`
}

type WalletUnfrozenData struct {
	WalletID models.ID `json:"wallet_id"`
	UserID   models.ID `json:"user_id"`
}

// Repository interfaces
type WalletRepository interface {
	Save(ctx context.Context, wallet *Wallet) error
	FindByID(ctx context.Context, id models.ID) (*Wallet, error)
	FindByUserID(ctx context.Context, userID models.ID) (*Wallet, error)
}

type TransactionRepository interface {
	Save(ctx context.Context, transaction *Transaction) error
	FindByID(ctx context.Context, id models.ID) (*Transaction, error)
	FindByWalletID(ctx context.Context, walletID models.ID, limit, offset int) ([]*Transaction, error)
	FindByPaymentID(ctx context.Context, paymentID models.ID) ([]*Transaction, error)
}

type MovementRepository interface {
	Save(ctx context.Context, movement *Movement) error
	FindByID(ctx context.Context, id models.ID) (*Movement, error)
	FindByWalletID(ctx context.Context, walletID models.ID, limit, offset int) ([]*Movement, error)
}