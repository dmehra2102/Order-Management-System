CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    customer TEXT NOT NULL,
    total_cents BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
)

CREATE TABLE IF NOT EXISTS order_items (
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku TEXT NOT NULL,
    quantity INT NOT NULL,
    price_cents BIGINT NOT NULL,
    PRIMARY KEY (order_id, sku)
)

CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    aggreagate_type TEXT NOT NULL,
    aggreagete_id TEXT NOT NULL,
    type TEXT NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    traceparent TEXT,
    created_at TIMESTAMPTZ NOT NUL DEFAULT now(),
    status TEXT NOT NULL DEFAULT 'pending',
    relay_id TEXT,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    lease_until TIMESTAMPTZ
)

CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(status) WHERE status = "pending";
CREATE INDEX IF NOT EXISTS idx_outbox_inprogress ON outbox(status) WHERE status = "in_progress";