package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/dmehra2102/Order-Management-System/internal/inventory/application"
	orderdom "github.com/dmehra2102/Order-Management-System/internal/order/domain"
	"github.com/dmehra2102/Order-Management-System/pkg/idempotency"
	"github.com/dmehra2102/Order-Management-System/pkg/tracing"
)

type Consumer struct {
	log    *slog.Logger
	reader *kafka.Reader
	svc    *application.Service
	idem   *idempotency.Store
	tracer trace.Tracer
}

func NewConsumer(log *slog.Logger, brokers []string, topic, group string, svc *application.Service, idem *idempotency.Store) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: group,
	})
	return &Consumer{
		log:    log,
		reader: r,
		svc:    svc,
		idem:   idem,
		tracer: otel.Tracer("inventory-consumer"),
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		key := c.idem.Key(msg.Topic, msg.Partition, msg.Offset)
		seen, err := c.idem.Seen(ctx, key)
		if err != nil {
			c.log.Error("idempotency check failed", "err", err)
			continue
		}
		if seen {
			c.log.Info("duplicate message skipped", "key", key)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		msgCtx := tracing.ExtractKafkaHeaders(ctx, msg.Headers)
		msgCtx, span := c.tracer.Start(msgCtx, "ConsumePaymentProcessed")

		var ev struct {
			OrderID     string `json:"OrderID"`
			AmountCents int64  `json:"AmountCents"`
		}
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			c.log.Error("unmarshal failed", "err", err)
			span.End()
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		// In a real system, fetch items from order DB; here I'm mocking with a single item
		items := []orderdom.OrderItem{{SKU: "SKU-1", Quantity: 2, PriceCents: 500}}
		headers := map[string]string{"source": "inventory-service"}
		traceparent := headerValue(msg.Headers, "traceparent")

		if err := c.svc.Reserve(msgCtx, ev.OrderID, items, headers, traceparent); err != nil {
			c.log.Error("reserve failed", "order_id", ev.OrderID, "err", err)
		} else {
			c.log.Info("stock reservation processed", "order_id", ev.OrderID)
		}
		span.End()
		_ = c.reader.CommitMessages(ctx, msg)
	}
}

func headerValue(h []kafka.Header, key string) string {
	for _, hh := range h {
		if hh.Key == key {
			return string(hh.Value)
		}
	}
	return ""
}
