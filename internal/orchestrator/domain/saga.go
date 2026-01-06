package domain

type SagaState string

const (
	StateStarted  SagaState = "started"
	StatePaid     SagaState = "paid"
	StateReserved SagaState = "reserved"
	StateCanceled SagaState = "canceled"
)

type Saga struct {
	OrderID string
	State   SagaState
}
