SHELL := /bin/bash

.PHONY: run migrate test build vet fmt clean

# Load KEY=VALUE pairs from .env into the shell environment. Values may contain
# special characters like `&` or `?`, so a plain `source .env` is not safe.
define load_env
while IFS='=' read -r key value; do \
	case "$$key" in '' | \#*) continue ;; esac; \
	export "$$key=$$value"; \
done < .env
endef

run: ## Start the API server (config reads .env itself)
	go run ./cmd/api

migrate: ## Apply pending migrations (config reads .env itself)
	go run ./cmd/api -migrate

test: ## Run all tests; integration tests use DATABASE_URL from .env
	@$(load_env); go test ./... -count=1

build: ## Compile all packages
	go build ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -l -w .

clean: ## Remove build artifacts
	rm -rf bin

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'
