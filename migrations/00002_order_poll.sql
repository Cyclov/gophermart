-- +goose Up
ALTER TABLE orders ADD COLUMN next_poll_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX orders_poll_idx ON orders(next_poll_at,number) WHERE status IN ('NEW','PROCESSING');

-- +goose Down
DROP INDEX orders_poll_idx;
ALTER TABLE orders DROP COLUMN next_poll_at;
