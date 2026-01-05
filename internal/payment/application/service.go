package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dmehra2102/Order-Management-System/internal/payment/domain"
)

type Service struct {
	repo PaymentRepository
}

func NewService(repo PaymentRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Process(ctx context.Context, orderID string, amount int64, headers map[string]string, traceparent string) error {
	// Mock payment: succeed if amount < 10_000 (₹10,000.00)
	p := domain.Payment{
		OrderID:     orderID,
		AmountCents: amount,
		Status:      domain.StatusProcessed,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	var eventType string
	var payload []byte

	if amount > 10_000 {
		p.Status = domain.StatusFailed
		event := domain.PaymentFailed{
			OrderID: orderID,
			Reason:  "amount too high",
		}
		eventType = "PaymentFailed"
		payload, _ = json.Marshal(event)
	} else {
		event := domain.PaymentProcessed{
			OrderID:     orderID,
			AmountCents: amount,
		}
		eventType = "PaymentProcessed"
		payload, _ = json.Marshal(event)
	}

	return s.repo.SaveWithOutbox(ctx, p, eventType, payload, headers, traceparent)
}
