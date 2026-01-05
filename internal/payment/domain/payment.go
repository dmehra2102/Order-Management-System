package domain

import "time"

type Status string

const (
	StatusProcessed Status = "processed"
	StatusFailed    Status = "failed"
)

type Payment struct {
	OrderID     string
	AmountCents int64
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
