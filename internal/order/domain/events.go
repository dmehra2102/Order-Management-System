package domain

type OrderCreated struct {
	OrderID    string
	Customer   string
	TotalCents int64
	Items      []OrderItem
}
