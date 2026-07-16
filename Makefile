SHELL := /bin/sh
.DEFAULT_GOAL := help

ENV_FILE ?= .env
COMPOSE := docker compose --env-file $(ENV_FILE)
BACKEND_DIR := backend
FRONTEND_DIR := frontend

.PHONY: help env check-env dev up down restart build test test-unit test-integration \
	test-e2e lint lint-go lint-frontend lint-openapi format migrate-up migrate-down \
	migrate-create migrate-status seed generate-openapi validate-compose logs clean smoke

help: ## Show documented targets
	@awk 'BEGIN {FS = ":.*## "; printf "BookFlow targets:\n\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

env: ## Create .env from the example with generated local secrets
	./scripts/bootstrap-env.sh "$(ENV_FILE)"

check-env: ## Verify that ENV_FILE exists and has no placeholders
	@./scripts/validate-env.sh "$(ENV_FILE)"

dev: check-env ## Build and run the app plus the complete local observability profile
	OTEL_ENABLED=true $(COMPOSE) --profile observability up --build

up: check-env ## Build and start the core stack in the background
	$(COMPOSE) up -d --build

down: ## Stop core and optional-profile containers without deleting data
	$(COMPOSE) --profile observability down --remove-orphans

restart: down up ## Restart the core stack

build: ## Build backend, frontend and container images
	cd $(BACKEND_DIR) && go build ./cmd/...
	cd $(FRONTEND_DIR) && npm run build
	$(COMPOSE) build api worker migrations frontend

test: test-unit test-integration ## Run unit and integration test suites

test-unit: ## Run Go race/short tests and frontend unit/component tests
	cd $(BACKEND_DIR) && go test -race -short ./...
	cd $(FRONTEND_DIR) && npm test

test-integration: ## Run integration-tagged Go tests (requires Docker for testcontainers)
	cd $(BACKEND_DIR) && go test -count=1 -tags=integration ./...

test-e2e: ## Run Playwright end-to-end tests
	cd $(FRONTEND_DIR) && npm run test:e2e

lint: lint-go lint-frontend lint-openapi validate-compose ## Run all static checks

lint-go: ## Check Go formatting, vet and golangci-lint
	@test -z "$$(gofmt -l $$(find $(BACKEND_DIR) -name '*.go' -type f))" || \
		(echo "Go files require gofmt"; gofmt -l $$(find $(BACKEND_DIR) -name '*.go' -type f); exit 1)
	cd $(BACKEND_DIR) && go vet ./...
	cd $(BACKEND_DIR) && golangci-lint run ./...

lint-frontend: ## Run ESLint, Prettier check and TypeScript typecheck
	cd $(FRONTEND_DIR) && npm run lint
	cd $(FRONTEND_DIR) && npm run format:check
	cd $(FRONTEND_DIR) && npm run typecheck

lint-openapi: ## Validate and lint the OpenAPI 3.1 contract
	./scripts/check-openapi-routes.py
	npx --yes @redocly/cli@2.39.0 lint backend/openapi/openapi.yaml

format: ## Format Go and frontend sources
	gofmt -w $$(find $(BACKEND_DIR) -name '*.go' -type f)
	cd $(FRONTEND_DIR) && npm run format

migrate-up: check-env ## Apply all pending database migrations
	$(COMPOSE) run --rm migrations /app/migrate up

migrate-down: check-env ## Revert one migration (destructive; confirm with DOWN_OK=1)
	@test "$(DOWN_OK)" = "1" || (echo "set DOWN_OK=1 to acknowledge rollback"; exit 1)
	$(COMPOSE) run --rm migrations /app/migrate down

migrate-create: ## Create paired SQL migration files; usage: make migrate-create name=add_table
	@test -n "$(name)" || (echo "usage: make migrate-create name=add_table"; exit 64)
	./scripts/create-migration.sh "$(name)"

migrate-status: check-env ## Show migration status
	$(COMPOSE) run --rm migrations /app/migrate status

seed: check-env ## Insert idempotent development seed data
	$(COMPOSE) run --rm migrations /app/seed

generate-openapi: ## Generate TypeScript types from the checked-in OpenAPI contract
	npx --yes openapi-typescript@7.9.1 backend/openapi/openapi.yaml -o frontend/src/api/schema.d.ts
	cd $(FRONTEND_DIR) && npx prettier --write src/api/schema.d.ts

validate-compose: ## Render the Compose model using ENV_FILE
	$(COMPOSE) --profile observability config --quiet

logs: ## Follow core service logs (override with services="api worker")
	$(COMPOSE) logs --follow --tail=200 $(or $(services),api worker frontend)

smoke: ## Check liveness, readiness and the frontend after the stack starts
	./scripts/smoke-test.sh

clean: ## Remove build/test artifacts; CLEAN_VOLUMES=1 also deletes local data
	rm -rf $(BACKEND_DIR)/bin $(BACKEND_DIR)/coverage.out $(BACKEND_DIR)/coverage.html
	rm -rf $(FRONTEND_DIR)/dist $(FRONTEND_DIR)/coverage $(FRONTEND_DIR)/playwright-report $(FRONTEND_DIR)/test-results
	@if [ "$(CLEAN_VOLUMES)" = "1" ]; then \
		$(COMPOSE) --profile observability down --volumes --remove-orphans; \
	else \
		echo "persistent volumes preserved (use CLEAN_VOLUMES=1 to delete them)"; \
	fi
