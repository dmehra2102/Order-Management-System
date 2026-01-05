package domain

type PaymentProcessed struct {
	OrderID     string
	AmountCents int64
}

type PaymentFailed struct {
	OrderID string
	Reason  string
}
