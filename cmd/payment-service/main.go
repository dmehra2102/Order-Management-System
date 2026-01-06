package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/dmehra2102/Order-Management-System/internal/payment/application"
	paymentkafka "github.com/dmehra2102/Order-Management-System/internal/payment/infrastructure/kafka"
	pg "github.com/dmehra2102/Order-Management-System/internal/payment/infrastructure/postgres"
	"github.com/dmehra2102/Order-Management-System/pkg/idempotency"
	"github.com/dmehra2102/Order-Management-System/pkg/logging"
	"github.com/dmehra2102/Order-Management-System/pkg/outbox"
	"github.com/dmehra2102/Order-Management-System/pkg/shutdown"
	"github.com/dmehra2102/Order-Management-System/pkg/tracing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

func main() {
	log := logging.New()
	ctx, cancel := shutdown.WithSignals(context.Background())
	defer cancel()

	pgURL := env("PG_URL", "postgres://postgres:postgres@localhost:5432/orderflow?sslmode=disable")
	kafkaAddr := env("KAFKA_ADDR", "localhost:9092")
	jaeger := env("JAEGER_URL", "http://localhost:14268/api/traces")
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	inTopic := env("IN_TOPIC", "order.events")
	outTopic := env("OUT_TOPIC", "payment.events")

	tp, err := tracing.Init(ctx, "payment-service", jaeger, log)
	if err != nil {
		log.Error("otel init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Error("pg connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisDB := redis.NewClient(&redis.Options{Addr: redisAddr})
	idem := idempotency.NewStore(redisDB, 10*time.Minute)

	repo := pg.NewRepository(log, pool)

	// Outbox relay for payment events
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaAddr),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
	}
	dispatch := outbox.NewDispatcher(log, writer, outTopic)
	store := orderOutboxStore{pool: pool, log: log}
	relay := outbox.NewRelay(log, store, dispatch, "payment-service-relay")
	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Error("relay stopped", "err", err)
		}
	}()

	svc := application.NewService(repo)
	consumer := paymentkafka.NewConsumer(log, []string{kafkaAddr}, inTopic, "payment-service", svc, idem)

	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("consumer stopped", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("payment-service shutdown")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Adapter to reuse other store methods for payment-service
type orderOutboxStore struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (s orderOutboxStore) LockBatch(ctx context.Context, relayID string, batchSize int, lease time.Duration) ([]outbox.Event, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx, `
		SELECT id, aggregate_type, aggregate_id, type, payload, headers, traceparent, created_at
		FROM outbox
		WHERE status = 'pending'
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []outbox.Event
	for rows.Next() {
		var event outbox.Event
		var headers map[string]string
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.Type, &event.Payload, &headers, &event.Traceparent, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Headers = headers
		events = append(events, event)
	}
	if len(events) == 0 {
		return nil, tx.Commit(ctx)
	}

	ids := make([]int64, 0, len(events))
	for _, ev := range events {
		ids = append(ids, ev.ID)
	}

	_, err = tx.Exec(ctx, ` UPDATE outbox SET status='in_progress', relay_id=$1, lease_until=now() + $2::interval WHERE id = ANY($3)`, relayID, lease.String(), ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

func (s orderOutboxStore) MarkSent(ctx context.Context, ids []int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='sent' WHERE id = ANY($1)`, ids)
	return err
}

func (s orderOutboxStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='failed', last_error=$2, retry_count=retry_count+1 WHERE id=$1`, id, errMsg)
	return err
}

func (s orderOutboxStore) ExtendLease(ctx context.Context, relayID string, ids []int64, lease time.Duration) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET lease_until=now() + $1::interval WHERE id = ANY($2) AND relay_id=$3`, lease.String(), ids, relayID)
	return err
}
