package application

import "context"

type Coordinator struct{}

func NewCoordinator() *Coordinator { return &Coordinator{} }

func (c *Coordinator) OnPaymentFailed(ctx context.Context, orderID string) error {
	// TODO: publish OrderCanceled
	return nil
}

func (c *Coordinator) OnStockFailed(ctx context.Context, orderID string) error {
	// TODO: publish RefundPayment & OrderCanceled
	return nil
}
