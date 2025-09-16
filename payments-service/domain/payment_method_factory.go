package domain

import (
	"errors"
	"fmt"
	"strings"
)

// PaymentMethodFactory creates payment methods based on type and creator with validation
type PaymentMethodFactory struct{}

// NewPaymentMethodFactory creates a new payment method factory
func NewPaymentMethodFactory() *PaymentMethodFactory {
	return &PaymentMethodFactory{}
}

// CreatePaymentMethod creates a payment method based on the type and creator with validation
func (f *PaymentMethodFactory) CreatePaymentMethod(paymentType PaymentMethodType, creator *PaymentMethodCreator) (*PaymentMethod, error) {
	if creator == nil {
		return nil, errors.New("payment method creator cannot be nil")
	}

	switch paymentType {
	case PaymentMethodTypeWallet:
		return f.createWalletPaymentMethod(creator)
	case PaymentMethodTypeCreditCard:
		return f.createCreditCardPaymentMethod(creator)
	case PaymentMethodTypeDebit:
		return f.createDebitPaymentMethod(creator)
	default:
		return nil, fmt.Errorf("unsupported payment method type: %s", paymentType.String())
	}
}

func (f *PaymentMethodFactory) createWalletPaymentMethod(creator *PaymentMethodCreator) (*PaymentMethod, error) {
	if creator.WalletID == nil {
		return nil, errors.New("wallet_id is required for wallet payment method")
	}

	if strings.TrimSpace(*creator.WalletID) == "" {
		return nil, errors.New("wallet_id cannot be empty")
	}

	return &PaymentMethod{
		PaymentMethodType: PaymentMethodTypeWallet,
		WalletPaymentMethod: &WalletPaymentMethod{
			WalletID: *creator.WalletID,
		},
	}, nil
}

func (f *PaymentMethodFactory) createCreditCardPaymentMethod(creator *PaymentMethodCreator) (*PaymentMethod, error) {
	if creator.CardToken == nil {
		return nil, errors.New("card_token is required for credit card payment method")
	}

	if strings.TrimSpace(*creator.CardToken) == "" {
		return nil, errors.New("card_token cannot be empty")
	}

	return &PaymentMethod{
		PaymentMethodType: PaymentMethodTypeCreditCard,
		CreditCardPaymentMethod: &CreditCardPaymentMethod{
			CardToken: *creator.CardToken,
		},
	}, nil
}

func (f *PaymentMethodFactory) createDebitPaymentMethod(creator *PaymentMethodCreator) (*PaymentMethod, error) {
	if creator.CardToken == nil {
		return nil, errors.New("card_token is required for debit payment method")
	}

	if strings.TrimSpace(*creator.CardToken) == "" {
		return nil, errors.New("card_token cannot be empty")
	}

	return &PaymentMethod{
		PaymentMethodType: PaymentMethodTypeDebit,
		CreditCardPaymentMethod: &CreditCardPaymentMethod{
			CardToken: *creator.CardToken,
		},
	}, nil
}
