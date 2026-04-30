.PHONY: run test migrate
DB ?= postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable

run:
	go run ./cmd/api

test:
	go test ./... -race

migrate:
	migrate -database "$(DB)" -path migrations up
