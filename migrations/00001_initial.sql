-- +goose Up
CREATE TABLE users (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 login TEXT NOT NULL UNIQUE CHECK (login <> ''),
 password_hash TEXT NOT NULL CHECK (password_hash <> ''),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE accounts (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),
 current NUMERIC NOT NULL DEFAULT 0 CHECK (current >= 0 AND current < 'Infinity'::numeric),
 withdrawn NUMERIC NOT NULL DEFAULT 0 CHECK (withdrawn >= 0 AND withdrawn < 'Infinity'::numeric)
);
CREATE TABLE orders (
 number TEXT PRIMARY KEY CHECK (number ~ '^[0-9]+$'),
 user_id BIGINT NOT NULL REFERENCES users(id),
 status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW','PROCESSING','INVALID','PROCESSED')),
 accrual NUMERIC CHECK (accrual >= 0 AND accrual < 'Infinity'::numeric),
 uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (status = 'PROCESSED' OR accrual IS NULL)
);
CREATE INDEX orders_user_uploaded_idx ON orders(user_id, uploaded_at DESC, number);
CREATE INDEX orders_pending_idx ON orders(uploaded_at, number) WHERE status IN ('NEW','PROCESSING');
CREATE TABLE withdrawals (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 order_number TEXT NOT NULL CHECK (order_number ~ '^[0-9]+$'),
 sum NUMERIC NOT NULL CHECK (sum > 0 AND sum < 'Infinity'::numeric),
 processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX withdrawals_user_processed_idx ON withdrawals(user_id, processed_at DESC, id DESC);

-- +goose Down
DROP TABLE withdrawals;
DROP TABLE orders;
DROP TABLE accounts;
DROP TABLE users;
