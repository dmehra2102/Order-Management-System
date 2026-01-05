package application

import (
	"context"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
)

type OrderRepository interface {
	SaveWithOutbox(ctx context.Context, o domain.Order, eventType string, payload []byte, headers map[string]string, traceparent string) error
	Get(ctx context.Context, id string) (domain.Order, error)
}

type InventoryClient interface {
	CheckStock(ctx context.Context, items []domain.OrderItem) (bool, error)
}
