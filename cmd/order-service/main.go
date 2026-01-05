package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/dmehra2102/Order-Management-System/pkg/logging"
	"github.com/dmehra2102/Order-Management-System/pkg/outbox"
	"github.com/dmehra2102/Order-Management-System/pkg/shutdown"
	"github.com/dmehra2102/Order-Management-System/pkg/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dmehra2102/Order-Management-System/internal/order/application"
	ordergrpc "github.com/dmehra2102/Order-Management-System/internal/order/infrastructure/grpc"
	orderhttp "github.com/dmehra2102/Order-Management-System/internal/order/infrastructure/http"
	orderkafka "github.com/dmehra2102/Order-Management-System/internal/order/infrastructure/kafka"
	orderpg "github.com/dmehra2102/Order-Management-System/internal/order/infrastructure/postgres"
)

func main() {
	log := logging.New()

	ctx, cancel := shutdown.WithSignals(context.Background())
	defer cancel()

	// Configuration
	pgURL := env("PG_URL", "postgres://postgres:postgres@localhost:5432/orderflow?sslmode=disable")
	kafkaBrokers := []string{env("KAFKA_ADDR", "localhost:9092")}
	jaeger := env("JAEGER_URL", "http://localhost:14268/api/traces")
	httpAddr := env("HTTP_ADDR", ":8080")
	outboxTopic := env("OUTBOX_TOPIC", "order.events")

	tp, err := tracing.Init(ctx, "order-service", jaeger, log)
	if err != nil {
		log.Error("otel init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	// Postgres Setup
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Error("pg connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Kafka producer
	writer := orderkafka.NewWriter(kafkaBrokers)
	defer writer.Close()

	// Repository & Outbox store
	repo := orderpg.NewRepository(log, pool)
	store := orderpg.NewOutboxStore(log, pool)
	dispatch := outbox.NewDispatcher(log, writer, outboxTopic)
	relay := outbox.NewRelay(log, store, dispatch, "order-service-relay")

	// Inventory client (mock for now)
	inv := ordergrpc.NewInventoryClient()
	svc := application.NewService(repo, inv)
	handler := orderhttp.NewHandler(log, svc)

	// HTTP server
	r := chi.NewRouter()
	r.Mount("/", handler.Routes())
	srv := &http.Server{
		Addr:         httpAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Run relay
	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Error("relay stopped with error", "err", err)
		}
	}()

	// Run HTTP
	go func() {
		log.Info("http listening", "addr", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	_ = srv.Shutdown(shutdownCtx)
	log.Info("order-service shutdown complete")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
