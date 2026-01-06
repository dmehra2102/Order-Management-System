package domain

type ReservationResult string

const (
	Reserved ReservationResult = "reserved"
	Failed   ReservationResult = "failed"
)

type StockReserved struct {
	OrderID string
}

type StockFailed struct {
	OrderID string
	Reason  string
}
