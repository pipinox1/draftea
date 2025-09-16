package domain

import (
	"errors"
	"fmt"
)

type PaymentMethodType string

const (
	PaymentMethodTypeCreditCard PaymentMethodType = "credit_card"
	PaymentMethodTypeDebit      PaymentMethodType = "debit"
	PaymentMethodTypeWallet     PaymentMethodType = "wallet"
)

var allPaymentMethodTypes = map[string]PaymentMethodType{
	PaymentMethodTypeCreditCard.String(): PaymentMethodTypeCreditCard,
	PaymentMethodTypeDebit.String():      PaymentMethodTypeDebit,
	PaymentMethodTypeWallet.String():     PaymentMethodTypeWallet,
}

func NewPaymentMethodType(value string) (*PaymentMethodType, error) {
	if value, ok := allPaymentMethodTypes[value]; ok {
		return &value, nil
	}
	return nil, errors.New(fmt.Sprintf("Unknown payment method type: %s", value))
}

func (pt PaymentMethodType) String() string {
	return string(pt)
}
