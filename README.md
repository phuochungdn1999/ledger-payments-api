# Ledger Payments API

[![CI/CD](https://github.com/phuochungdn1999/ledger-payments-api/actions/workflows/ci.yml/badge.svg)](https://github.com/phuochungdn1999/ledger-payments-api/actions/workflows/ci.yml)


A double-entry-bookkeeping payments service in Go. Every transfer is recorded as balanced debit/credit postings inside a single Postgres transaction, so the books always balance — no money is ever invented or destroyed.

## Why this matters for fintech
- **Double-entry ledger** — the schema makes "I have N dollars" a derived view, not a stored field. Bugs that would corrupt a "balance" column become impossible.
- **Idempotency keys** — clients can retry safely. The same key + same body returns the original response; the same key + different body is rejected.
- **ACID transfers** — debit and credit posted in the same SQL transaction with `SERIALIZABLE` isolation. Account version checks prevent stale reads.
- **Webhooks** — every settled transfer fires an outbound webhook with HMAC-SHA256 signature; deliveries are retried with exponential backoff and dead-lettered after N attempts.
- **Tested** — unit tests cover idempotency, signature verification, and the invariant "sum of postings always = 0".

## Stack
Go 1.22 · chi · pgx · Postgres 16 · stretchr/testify

## Run
```bash
docker compose up -d postgres
make migrate
make run        # :8080
make test
```

## Endpoints
| Method | Path                | Notes                                  |
|--------|---------------------|----------------------------------------|
| POST   | /v1/accounts        | Create an account                      |
| GET    | /v1/accounts/:id    | Account + computed balance             |
| POST   | /v1/transfers       | Move money. Requires `Idempotency-Key` |
| GET    | /v1/transfers/:id   | Transfer with postings                 |
| POST   | /v1/webhooks        | Register a webhook subscriber          |

## Invariants
- For any transfer T: `SUM(postings.amount WHERE transfer_id = T) = 0`
- For any account: `balance = SUM(postings.amount WHERE account_id = account)`
- Postings are append-only; corrections happen via reversing transfers, never UPDATE/DELETE.
