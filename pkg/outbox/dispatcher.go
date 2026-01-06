package outbox

import (
	"context"
	"errors"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type Producer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type Dispatcher struct {
	log      *slog.Logger
	producer Producer
	topic    string
}

func NewDispatcher(log *slog.Logger, producer Producer, topic string) *Dispatcher {
	return &Dispatcher{log: log, producer: producer, topic: topic}
}

func (d *Dispatcher) Disptach(ctx context.Context, event Event) error {
	headers := make([]kafka.Header, 0, len(event.Headers)+1)

	for k, v := range event.Headers {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	headers = append(headers, kafka.Header{Key: "event_type", Value: []byte(event.Type)})
	if event.Traceparent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(event.Traceparent)})
	}

	msg := kafka.Message{
		Topic:   d.topic,
		Key:     []byte(event.AggregateID),
		Value:   event.Payload,
		Headers: headers,
	}
	if err := d.producer.WriteMessages(ctx, msg); err != nil {
		d.log.Error("outbox dispatch failed", "event_id", event.ID, "err", err)
		return err
	}
	d.log.Info("outbox dispatched", "event_id", event.ID, "type", event.Type)
	return nil
}

var ErrPermanent = errors.New("permanent")
