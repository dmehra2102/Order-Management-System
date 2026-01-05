package application

import (
	"context"

	"github.com/dmehra2102/Order-Management-System/internal/payment/domain"
)

type PaymentRepository interface {
	SaveWithOutbox(ctx context.Context, p domain.Payment, eventType string, payload []byte, headers map[string]string, traceparent string) error
}

type PaymentPublisher interface {
	// TODO
}
