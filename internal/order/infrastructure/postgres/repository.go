package postgres

import (
	"context"
	"log/slog"

	"github.com/dmehra2102/Order-Management-System/internal/order/domain"
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
