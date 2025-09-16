package domain

// PaymentMethodCreator contains all possible fields for creating payment methods
// Fields are pointers to allow nil checking for validation
type PaymentMethodCreator struct {
	// Wallet payment fields
	WalletID *string

	// Credit card/debit payment fields
	CardToken *string
}

// NewWalletPaymentCreator creates a creator for wallet payments
func NewWalletPaymentCreator(walletID string) *PaymentMethodCreator {
	return &PaymentMethodCreator{
		WalletID: &walletID,
	}
}

// NewCreditCardPaymentCreator creates a creator for credit card payments
func NewCreditCardPaymentCreator(cardToken string) *PaymentMethodCreator {
	return &PaymentMethodCreator{
		CardToken: &cardToken,
	}
}

// NewDebitPaymentCreator creates a creator for debit payments
func NewDebitPaymentCreator(cardToken string) *PaymentMethodCreator {
	return &PaymentMethodCreator{
		CardToken: &cardToken,
	}
}