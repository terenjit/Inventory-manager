DB_DSN       := "mysql://root:secret@tcp(localhost:3306)/textile_pos"
DB_TEST_DSN  := "mysql://root:secret@tcp(localhost:3307)/textile_pos_test"
MIGRATE      := go run -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
MIGRATIONS   := ./migrations

.PHONY: up down migrate-up migrate-down migrate-test-up run build test lint

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS) -database $(DB_DSN) up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS) -database $(DB_DSN) down 1

migrate-test-up:
	$(MIGRATE) -path $(MIGRATIONS) -database $(DB_TEST_DSN) up

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -count=1

seed:
	go run ./cmd/seed

check-ledger:
	go run ./cmd/checkledger

lint:
	golangci-lint run ./...
