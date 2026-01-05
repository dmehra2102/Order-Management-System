package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Writer struct {
	*kafka.Writer
}

func NewWriter(brokers []string) *Writer {
	return &Writer{
		Writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

func (w *Writer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return w.Writer.WriteMessages(ctx, msgs...)
}
