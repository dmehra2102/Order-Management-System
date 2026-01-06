package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

func NewRepository(log *slog.Logger, pool *pgxpool.Pool) *Repository {
	return &Repository{
		log:  log,
		pool: pool,
	}
}

func (r *Repository) ReserveWithOutbox(ctx context.Context, orderID string, items []domain.OrderItem, ok bool, eventType string, payload []byte, headers map[string]string, traceparent string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS reservations ( 
		order_id TEXT PRIMARY KEY, 
		ok BOOLEAN NOT NULL, 
		created_at TIMESTAMPTZ NOT NULL, 
		updated_at TIMESTAMPTZ NOT NULL 
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO reservations (order_id, ok, created_at, updated_at) VALUES ($1,$2,$3,$4) ON CONFLICT (order_id) DO UPDATE SET ok=$2, updated_at=$4`,
		orderID, ok, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox (aggregate_type, aggregate_id, type, payload, headers,
		traceparent, status) VALUES ($1,$2,$3,$4,$5,$6,'pending')`,
		"inventory", orderID, eventType, payload, headers, traceparent)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
