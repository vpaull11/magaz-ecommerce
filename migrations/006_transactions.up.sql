-- Payment microservice transactions table
-- (runs in its own DB / schema, tracked separately)
CREATE TABLE IF NOT EXISTS transactions (
    id          VARCHAR(36)    PRIMARY KEY, -- UUID
    order_id    BIGINT         NOT NULL,
    amount      NUMERIC(10, 2) NOT NULL,
    card_last4  VARCHAR(4)     NOT NULL,
    status      VARCHAR(50)    NOT NULL DEFAULT 'pending',
    message     VARCHAR(255)   NOT NULL DEFAULT '',
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tx_order ON transactions(order_id);
