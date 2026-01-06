package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"

	"github.com/dmehra2102/Order-Management-System/internal/inventory/application"
	invgrpc "github.com/dmehra2102/Order-Management-System/internal/inventory/infrastructure/grpc"
	inventoryKafka "github.com/dmehra2102/Order-Management-System/internal/inventory/infrastructure/kafka"
	inventoryDB "github.com/dmehra2102/Order-Management-System/internal/inventory/infrastructure/postgres"
	"github.com/dmehra2102/Order-Management-System/pkg/idempotency"
	"github.com/dmehra2102/Order-Management-System/pkg/logging"
	"github.com/dmehra2102/Order-Management-System/pkg/outbox"
	"github.com/dmehra2102/Order-Management-System/pkg/shutdown"
	"github.com/dmehra2102/Order-Management-System/pkg/tracing"
)

func main() {
	log := logging.New()
	ctx, cancel := shutdown.WithSignals(context.Background())
	defer cancel()

	pgURL := env("PG_URL", "postgres://postgres:postgres@localhost:5432/orderflow?sslmode=disable")
	kafkaAddr := env("KAFKA_ADDR", "localhost:9092")
	jaeger := env("JAEGER_URL", "http://localhost:14268/api/traces")
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	inTopic := env("IN_TOPIC", "payment.events")
	outTopic := env("OUT_TOPIC", "inventory.events")
	grpcAddr := env("GRPC_ADDR", ":50051")

	tp, err := tracing.Init(ctx, "inventory-service", jaeger, log)
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

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	idem := idempotency.NewStore(rdb, 10*time.Minute)

	repo := inventoryDB.NewRepository(log, pool)

	// Outbox relay
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaAddr),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
	}
	dispatch := outbox.NewDispatcher(log, writer, outTopic)
	store := inventoryOutboxStore{pool: pool, log: log}
	relay := outbox.NewRelay(log, store, dispatch, "inventory-service-relay")
	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Error("relay stopped", "err", err)
		}
	}()

	// gRPC server
	gs, err := invgrpc.Run(grpcAddr, invgrpc.NewServer())
	if err != nil {
		log.Error("grpc server failed", "err", err)
		os.Exit(1)
	}
	defer gs.GracefulStop()

	svc := application.NewService(repo)
	consumer := inventoryKafka.NewConsumer(log, []string{kafkaAddr}, inTopic, "inventory-service", svc, idem)

	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("consumer stopped", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("inventory-service shutdown")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

type inventoryOutboxStore struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (s inventoryOutboxStore) LockBatch(ctx context.Context, relayID string, batchSize int, lease time.Duration) ([]outbox.Event, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, aggregate_type, aggregate_id, type, payload, headers, traceparent, created_at
        FROM outbox
        WHERE status='pending'
        ORDER BY id
        FOR UPDATE SKIP LOCKED
        LIMIT $1`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var events []outbox.Event
	for rows.Next() {
		var ev outbox.Event
		var headers map[string]string
		if err := rows.Scan(&ev.ID, &ev.AggregateType, &ev.AggregateID, &ev.Type, &ev.Payload, &headers, &ev.Traceparent, &ev.CreatedAt); err != nil {
			return nil, err
		}
		ev.Headers = headers
		events = append(events, ev)
	}
	if len(events) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(events))
	for _, ev := range events {
		ids = append(ids, ev.ID)
	}
	_, err = s.pool.Exec(ctx, `
        UPDATE outbox
        SET status='in_progress', relay_id=$1, lease_until=now() + $2::interval
        WHERE id = ANY($3)`, relayID, lease.String(), ids)
	return events, err
}

func (s inventoryOutboxStore) MarkSent(ctx context.Context, ids []int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='sent' WHERE id = ANY($1)`, ids)
	return err
}

func (s inventoryOutboxStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='failed', last_error=$2, retry_count=retry_count+1 WHERE id=$1`, id, errMsg)
	return err
}

func (s inventoryOutboxStore) ExtendLease(ctx context.Context, relayID string, ids []int64, lease time.Duration) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET lease_until=now() + $1::interval WHERE id = ANY($2) AND relay_id=$3`, lease.String(), ids, relayID)
	return err
}
