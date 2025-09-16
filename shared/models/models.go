package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ID represents a unique identifier
type ID string

// GenerateUUID creates a new UUID
func GenerateUUID() ID {
	return ID(uuid.New().String())
}

// NewID creates an ID from string
func NewID(id string) (ID, error) {
	_, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}
	return ID(id), nil
}

// String returns string representation
func (id ID) String() string {
	return string(id)
}

// Timestamps represents creation and update times
type Timestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// NewTimestamps creates new timestamps
func NewTimestamps() Timestamps {
	now := time.Now()
	return Timestamps{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Update updates the UpdatedAt timestamp
func (t Timestamps) Update() Timestamps {
	t.UpdatedAt = time.Now()
	return t
}

// Version represents entity version for optimistic locking
type Version struct {
	Value int
}

// NewVersion creates new version
func NewVersion() Version {
	return Version{Value: 1}
}

// Update increments version
func (v Version) Update() Version {
	v.Value++
	return v
}

// Money represents monetary amount
type Money struct {
	Amount   int64  `json:"amount"`   // Amount in cents
	Currency string `json:"currency"` // Currency code (USD, EUR, etc.)
}

// NewMoney creates a new money value
func NewMoney(amount int64, currency string) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

// IsZero checks if money is zero
func (m Money) IsZero() bool {
	return m.Amount == 0
}

// IsPositive checks if money is positive
func (m Money) IsPositive() bool {
	return m.Amount > 0
}

// Add adds two money values (must have same currency)
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("currency mismatch")
	}
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}, nil
}

// Subtract subtracts two money values (must have same currency)
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("currency mismatch")
	}
	return Money{
		Amount:   m.Amount - other.Amount,
		Currency: m.Currency,
	}, nil
}