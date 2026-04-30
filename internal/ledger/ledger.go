// Package ledger implements a double-entry bookkeeping core.
//
// Money never lives in a single column you can mutate; instead, every
// movement is two postings (a debit and a credit) that sum to zero. An
// account's balance is the SUM of its postings — derived, not stored.
//
// All transfers run inside a single SERIALIZABLE Postgres transaction
// so we never see torn writes between the debit and credit.
package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Account struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Currency string `json:"currency"`
	Balance  int64  `json:"balance"` // computed
}

type Transfer struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Description string `json:"description"`
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

type Service struct{ pool *pgxpool.Pool }

func New(p *pgxpool.Pool) *Service { return &Service{pool: p} }

func (s *Service) CreateAccount(ctx context.Context, owner, currency string) (*Account, error) {
	if len(currency) != 3 {
		return nil, errors.New("currency must be 3 letters")
	}
	a := &Account{ID: uuid.NewString(), Owner: owner, Currency: currency}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO accounts(id, owner, currency) VALUES($1,$2,$3)`,
		a.ID, a.Owner, a.Currency)
	return a, err
}

func (s *Service) GetAccount(ctx context.Context, id string) (*Account, error) {
	a := &Account{ID: id}
	err := s.pool.QueryRow(ctx, `
		SELECT a.owner, a.currency, COALESCE(SUM(p.amount), 0)
		FROM accounts a LEFT JOIN postings p ON p.account_id = a.id
		WHERE a.id = $1 GROUP BY a.id`, id).Scan(&a.Owner, &a.Currency, &a.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrCurrencyMismatch  = errors.New("currency mismatch")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different body")
)

// Transfer atomically debits `from` and credits `to`. Idempotent on key.
func (s *Service) Transfer(ctx context.Context, key string, t Transfer) (*Transfer, error) {
	if t.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if t.From == t.To {
		return nil, errors.New("from and to must differ")
	}

	hash := requestHash(t)

	// Idempotency check first. If the key exists with the same hash, return
	// the existing record. If hashes differ, conflict.
	if existing, h, err := s.lookupTransfer(ctx, key); err == nil {
		if h != hash {
			return nil, ErrIdempotencyConflict
		}
		return existing, nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var fromCurrency, toCurrency string
	if err := tx.QueryRow(ctx, `SELECT currency FROM accounts WHERE id=$1`, t.From).Scan(&fromCurrency); err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT currency FROM accounts WHERE id=$1`, t.To).Scan(&toCurrency); err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	if fromCurrency != toCurrency || fromCurrency != t.Currency {
		return nil, ErrCurrencyMismatch
	}

	var bal int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM postings WHERE account_id=$1`, t.From).Scan(&bal); err != nil {
		return nil, err
	}
	if bal < t.Amount {
		return nil, ErrInsufficientFunds
	}

	t.ID = uuid.NewString()
	t.Status = "settled"
	if _, err := tx.Exec(ctx,
		`INSERT INTO transfers(id,status,description,idempotency_key,request_hash)
		 VALUES($1,$2,$3,$4,$5)`,
		t.ID, t.Status, t.Description, key, hash); err != nil {
		return nil, err
	}

	// Two postings, sum = 0. Bookkeeping invariant.
	batch := &pgx.Batch{}
	batch.Queue(`INSERT INTO postings(transfer_id, account_id, amount, currency) VALUES($1,$2,$3,$4)`,
		t.ID, t.From, -t.Amount, t.Currency)
	batch.Queue(`INSERT INTO postings(transfer_id, account_id, amount, currency) VALUES($1,$2,$3,$4)`,
		t.ID, t.To, t.Amount, t.Currency)
	br := tx.SendBatch(ctx, batch)
	for i := 0; i < 2; i++ {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return nil, err
		}
	}
	br.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) lookupTransfer(ctx context.Context, key string) (*Transfer, string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT t.id, t.status, COALESCE(t.description,''), t.request_hash,
			(SELECT account_id FROM postings WHERE transfer_id=t.id AND amount<0),
			(SELECT account_id FROM postings WHERE transfer_id=t.id AND amount>0),
			(SELECT amount FROM postings WHERE transfer_id=t.id AND amount>0),
			(SELECT currency FROM postings WHERE transfer_id=t.id LIMIT 1)
		FROM transfers t WHERE t.idempotency_key=$1`, key)
	var t Transfer
	var hash string
	if err := row.Scan(&t.ID, &t.Status, &t.Description, &hash,
		&t.From, &t.To, &t.Amount, &t.Currency); err != nil {
		return nil, "", err
	}
	return &t, hash, nil
}

func requestHash(t Transfer) string {
	body := struct {
		From, To, Currency string
		Amount             int64
	}{t.From, t.To, t.Currency, t.Amount}
	b, _ := json.Marshal(body)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
