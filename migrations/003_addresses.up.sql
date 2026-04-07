CREATE TABLE IF NOT EXISTS addresses (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label      VARCHAR(255) NOT NULL DEFAULT 'Домашний',
    city       VARCHAR(255) NOT NULL,
    street     VARCHAR(500) NOT NULL,
    zip        VARCHAR(20)  NOT NULL,
    is_default BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_addresses_user ON addresses(user_id);
