package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
	"github.com/dmehra2102/Order-Management-System/pkg/outbox"
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

func (r *Repository) SaveWithOutbox(ctx context.Context, o domain.Order, eventType string, payload []byte, headers map[string]string, traceparent string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `INSERT INTO orders (id, customer, total_cents, status, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (id) DO UPDATE SET customer=$2,total_cents=$3,status=$4,updated_at=$6
			`)
	if err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for _, item := range o.Items {
		batch.Queue(`INSERT INTO order_items (order_id, sku, quantity, price_cents)
            VALUES ($1,$2,$3,$4)
            ON CONFLICT (order_id, sku) DO UPDATE SET quantity=$3, price_cents=$4`,
			o.ID, item.SKU, item.Quantity, item.PriceCents)
	}
	batchResult := tx.SendBatch(ctx, batch)
	if err = batchResult.Close(); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INERT INTO outbox (aggregate_type, aggregate_id, type, payload, headers, traceparent, status)  VALUES ($1,$2,$3,$4,$5,$6,'pending')`,
		"order", o.ID, eventType, payload, headers, traceparent)

	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Order, error) {
	var o domain.Order
	err := r.pool.QueryRow(ctx, `SELECT id, customer, total_cents, status, created_at, updated_at FROM orders WHERE id=$1`, id).
		Scan(&o.ID, &o.Customer, &o.TotalCents, &o.Status, &o.CreatedAt, &o.UpdatedAt)

	if err != nil {
		return domain.Order{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT sku, quantity, price_cents FROM order_items WHERE order_id=$1`, id)
	if err != nil {
		return domain.Order{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var sku string
		var qty int
		var price int64
		if err := rows.Scan(&sku, &qty, &price); err != nil {
			return domain.Order{}, err
		}
		o.Items = append(o.Items, domain.OrderItem{SKU: sku, Quantity: qty, PriceCents: price})
	}
	return o, nil
}

type OutboxStore struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

func NewOutboxStore(log *slog.Logger, pool *pgxpool.Pool) *OutboxStore {
	return &OutboxStore{log: log, pool: pool}
}

func (s *OutboxStore) LockBatch(ctx context.Context, relayID string, batchSize int, lease time.Duration) ([]outbox.Event, error) {
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

func (s *OutboxStore) MarkSent(ctx context.Context, ids []int64) error {
	ct, err := s.pool.Exec(ctx, `UPDATE outbox SET status='sent' WHERE id = ANY($1)`, ids)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("no rows updated")
	}
	return nil
}

func (s *OutboxStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='failed', last_error=$2, retry_count=retry_count+1 WHERE id=$1`, id, errMsg)
	return err
}

func (s *OutboxStore) ExtendLease(ctx context.Context, relayID string, ids []int64, lease time.Duration) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET lease_until=now() + $1::interval WHERE id = ANY($2) AND relay_id=$3`, lease.String(), ids, relayID)
	return err
}
