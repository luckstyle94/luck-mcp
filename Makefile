.PHONY: up down test fmt migrate lint

GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

up:
	docker compose up -d

down:
	docker compose down

test:
	go test ./...

fmt:
	gofmt -w $(GOFILES)
	@if command -v goimports >/dev/null 2>&1; then goimports -w $(GOFILES); else echo "goimports nao encontrado, pulando"; fi

migrate:
	docker compose exec -T postgres psql -U mcp -d mcp -f /migrations/0001_init.up.sql

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "golangci-lint nao encontrado, pulando"; fi
