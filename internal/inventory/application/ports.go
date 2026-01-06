package application

import (
	"context"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
)

type StockRepository interface {
	ReserveWithOutbox(ctx context.Context, orderID string, items []domain.OrderItem, ok bool, eventType string, payload []byte, headers map[string]string, traceparent string) error
}
