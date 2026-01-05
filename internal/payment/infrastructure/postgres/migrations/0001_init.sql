CREATE TABLE IF NOT EXISTS payments (
    order_id TEXT PRIMARY KEY,
    amount_cents BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
)