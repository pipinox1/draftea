package domain

// PaymentMethod represents a payment method with type-specific data
type PaymentMethod struct {
	PaymentMethodType PaymentMethodType
	*WalletPaymentMethod
	*CreditCardPaymentMethod
}

// NewPaymentMethod creates a new payment method using the factory and creator
func NewPaymentMethod(paymentType PaymentMethodType, creator *PaymentMethodCreator) (*PaymentMethod, error) {
	factory := NewPaymentMethodFactory()
	return factory.CreatePaymentMethod(paymentType, creator)
}

type CreditCardPaymentMethod struct {
	CardToken string
}

type WalletPaymentMethod struct {
	WalletID string
}
