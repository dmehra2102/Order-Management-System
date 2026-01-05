package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const TraceparentHeader = "traceparent"

func InjectKafkaHeaders(ctx context.Context, headers []kafka.Header) []kafka.Header {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	return headers
}

func ExtractKafkaHeaders(ctx context.Context, headers []kafka.Header) context.Context {
	carrier := propagation.MapCarrier{}

	for _, h := range headers {
		carrier[h.Key] = string(h.Value)
	}

	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
