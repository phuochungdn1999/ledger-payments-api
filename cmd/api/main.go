package main

import (
	"context"
	"log"
	"net/http"
	"os"

	httpapi "github.com/binn/ledger-payments-api/internal/http"
	"github.com/binn/ledger-payments-api/internal/ledger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	svc := ledger.New(pool)
	addr := env("ADDR", ":8080")
	log.Printf("ledger-payments-api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New(svc)))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
