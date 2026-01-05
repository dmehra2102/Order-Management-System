package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
)

var ErrStockUnavailable = errors.New("stock unavilable")

type Service struct {
	repo OrderRepository
	inv  InventoryClient
}

func NewService(repo OrderRepository, inv InventoryClient) *Service {
	return &Service{repo: repo, inv: inv}
}

func (s *Service) CreateOrder(ctx context.Context, o domain.Order, headers map[string]string, traceparent string) error {
	ok, err := s.inv.CheckStock(ctx, o.Items)
	if err != nil {
		return err
	}
	if !ok {
		return ErrStockUnavailable
	}

	event := domain.OrderCreated{
		OrderID:    o.ID,
		Customer:   o.Customer,
		TotalCents: o.TotalCents,
		Items:      o.Items,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.repo.SaveWithOutbox(ctx, o, "OrderCreated", payload, headers, traceparent)
}
