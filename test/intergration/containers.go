package intergration

import (
	"context"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type Env struct {
	PG     *postgres.PostgresContainer
	Kafka  *kafka.KafkaContainer
	PGURL  string
	KAddr  []string
	Cancel context.CancelFunc
}

func Setup(ctx context.Context) (*Env, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("orderflow"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	pgURL, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		return nil, err
	}

	kafkaC, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"), // TODO: Change this cluster ID with yours
	)
	if err != nil {
		cancel()
		return nil, err
	}

	kafkaAddress, err := kafkaC.Brokers(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Env{
		PG:     pgC,
		Kafka:  kafkaC,
		PGURL:  pgURL,
		KAddr:  kafkaAddress,
		Cancel: cancel,
	}, nil
}

func (e *Env) Teardown(ctx context.Context) {
	e.Cancel()
	_ = e.Kafka.Terminate(ctx)
	_ = e.PG.Terminate(ctx)
}
