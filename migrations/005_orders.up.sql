CREATE TABLE IF NOT EXISTS orders (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT         NOT NULL REFERENCES users(id),
    address_id     BIGINT         REFERENCES addresses(id),
    status         VARCHAR(50)    NOT NULL DEFAULT 'pending',
    total_amount   NUMERIC(10, 2) NOT NULL,
    payment_tx_id  VARCHAR(255),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS order_items (
    id             BIGSERIAL PRIMARY KEY,
    order_id       BIGINT         NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id     BIGINT         NOT NULL REFERENCES products(id),
    product_name   VARCHAR(255)   NOT NULL,
    quantity       INTEGER        NOT NULL,
    price_snapshot NUMERIC(10, 2) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_user   ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
