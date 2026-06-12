.PHONY: build test test-integration lint vuln sqlc migrate docker

BINARY := bin/server
MIGRATION_DIR := migrations
DB_URL ?= $(DATABASE_URL)

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

test:
	go test -race ./...

test-integration:
	go test -tags=integration -race ./tests/integration/... ./tests/e2e/...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

sqlc:
	sqlc generate

migrate:
	migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" down

docker:
	docker build -f deploy/Dockerfile -t feature-voting-system:latest .
