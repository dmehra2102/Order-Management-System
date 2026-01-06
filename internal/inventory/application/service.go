package application

import (
	"context"
	"encoding/json"

	inventoryDomain "github.com/dmehra2102/Order-Management-System/internal/inventory/domain"
	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
)

type Service struct {
	repo StockRepository
}

func NewService(repo StockRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Reserve(ctx context.Context, orderID string, items []domain.OrderItem, headers map[string]string, traceparent string) error {
	// Mocking reservation: fail if any quantity > 10
	ok := true
	for _, item := range items {
		if item.Quantity > 10 {
			ok = false
			break
		}
	}
	var eventType string
	var payload []byte
	if ok {
		eventType = "StockReserved"
		payload, _ = json.Marshal(inventoryDomain.StockReserved{OrderID: orderID})
	} else {
		eventType = "StockFailed"
		payload, _ = json.Marshal(inventoryDomain.StockFailed{OrderID: orderID, Reason: "insufficient stock"})
	}
	return s.repo.ReserveWithOutbox(ctx, orderID, items, ok, eventType, payload, headers, traceparent)
}
