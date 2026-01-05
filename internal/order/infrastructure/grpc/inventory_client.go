package grpc

import (
	"context"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
)

type InventoryClient struct{}

func NewInventoryClient() *InventoryClient {
	return &InventoryClient{}
}

// Todo : Need to implement this later (at this moment inventory didn't exists).
func (c *InventoryClient) CheckStock(ctx context.Context, items []domain.OrderItem) (bool, error) {
	return true, nil
}
