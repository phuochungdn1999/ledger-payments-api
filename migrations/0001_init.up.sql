CREATE TABLE IF NOT EXISTS accounts (
  id          UUID PRIMARY KEY,
  owner       TEXT NOT NULL,
  currency    TEXT NOT NULL CHECK (length(currency) = 3),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transfers (
  id            UUID PRIMARY KEY,
  status        TEXT NOT NULL CHECK (status IN ('pending','settled','failed')),
  description   TEXT,
  idempotency_key TEXT NOT NULL,
  request_hash    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Same key + same request body -> dedupe and return original response.
-- Same key + different body -> conflict.
CREATE UNIQUE INDEX IF NOT EXISTS transfers_idem_uk ON transfers(idempotency_key);

-- Postings: append-only, signed amounts. SUM per transfer = 0.
CREATE TABLE IF NOT EXISTS postings (
  id           BIGSERIAL PRIMARY KEY,
  transfer_id  UUID NOT NULL REFERENCES transfers(id),
  account_id   UUID NOT NULL REFERENCES accounts(id),
  amount       BIGINT NOT NULL,
  currency     TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS postings_account_idx ON postings(account_id);
CREATE INDEX IF NOT EXISTS postings_transfer_idx ON postings(transfer_id);

CREATE TABLE IF NOT EXISTS webhook_subs (
  id          UUID PRIMARY KEY,
  url         TEXT NOT NULL,
  secret      TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id            BIGSERIAL PRIMARY KEY,
  sub_id        UUID NOT NULL REFERENCES webhook_subs(id),
  event_type    TEXT NOT NULL,
  payload       JSONB NOT NULL,
  attempts      INT NOT NULL DEFAULT 0,
  next_attempt  TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at  TIMESTAMPTZ,
  failed_at     TIMESTAMPTZ
);
