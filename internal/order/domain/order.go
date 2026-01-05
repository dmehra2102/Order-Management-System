package domain

import "time"

type OrderStatus string

const (
	StatusPending  OrderStatus = "pending"
	StatusPaid     OrderStatus = "paid"
	StatusReserved OrderStatus = "reserved"
	StatusCanceled OrderStatus = "canceled"
)

type Order struct {
	ID         string
	Customer   string
	Items      []OrderItem
	TotalCents int64
	Status     OrderStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type OrderItem struct {
	SKU        string
	Quantity   int
	PriceCents int64
}

func NewOrder(id, customer string, items []OrderItem) Order {
	var total int64
	for _, item := range items {
		total += int64(item.Quantity) * item.PriceCents
	}
	now := time.Now().UTC()
	return Order{
		ID:         id,
		Customer:   customer,
		Items:      items,
		TotalCents: total,
		Status:     StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
