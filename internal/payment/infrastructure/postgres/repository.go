package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/dmehra2102/Order-Management-System/internal/payment/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

func NewRepository(log *slog.Logger, pool *pgxpool.Pool) *Repository {
	return &Repository{log: log, pool: pool}
}

func (r *Repository) SaveWithOutbox(ctx context.Context, p domain.Payment, eventType string, payload []byte, headers map[string]string, traceparent string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `INSERT INTO payments (order_id, amount_cents, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (order_id) DO UPDATE SET amount_cents=$2,status=$3,updated_at=$5`,
		p.OrderID, p.AmountCents, p.Status, p.CreatedAt, time.Now().UTC())
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO outbox (aggregate_type, aggregate_id, type, payload, headers, traceparent, status) VALUES ($1,$2,$3,$4,$5,$6,'pending')`, "payment",
		p.OrderID, eventType, payload, headers, traceparent)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
