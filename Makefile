.PHONY: up down test fmt migrate lint health index index-full

GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
PROJECT ?=
ROOT ?=
MODE ?= changed

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
	docker compose exec -T mcp /mcp-server migrate

health:
	docker compose run --rm -T --no-deps \
		--entrypoint /mcp-server \
		-e DATABASE_URL=postgres://mcp:mcp@postgres:5432/mcp?sslmode=disable \
		-e OLLAMA_URL=http://ollama:11434 \
		-e OLLAMA_EMBED_MODEL=nomic-embed-text \
		-e LOG_LEVEL=info \
		mcp health

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "golangci-lint nao encontrado, pulando"; fi

index:
	@if [ -z "$(PROJECT)" ]; then echo "PROJECT obrigatorio. Ex: make index PROJECT=meu-projeto ROOT=/caminho/do/repo"; exit 1; fi
	@if [ -z "$(ROOT)" ]; then echo "ROOT obrigatorio. Ex: make index PROJECT=meu-projeto ROOT=/caminho/do/repo"; exit 1; fi
	docker compose run --rm -T --no-deps \
		--entrypoint /mcp-server \
		-e DATABASE_URL=postgres://mcp:mcp@postgres:5432/mcp?sslmode=disable \
		-e OLLAMA_URL=http://ollama:11434 \
		-e OLLAMA_EMBED_MODEL=nomic-embed-text \
		-e LOG_LEVEL=info \
		-v "$(ROOT)":/workspace/index:ro \
		mcp index --project "$(PROJECT)" --root /workspace/index --mode "$(MODE)"

index-full:
	$(MAKE) index PROJECT="$(PROJECT)" ROOT="$(ROOT)" MODE=full
