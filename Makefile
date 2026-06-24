# Flashbacks - Common Makefile
# ============================
# Provides unified commands for all microservices.
# Usage: make <command> [SERVICE=<service-name>]

SHELL := /bin/bash

# Default service (can be overridden)
SERVICE ?= api-service

# Docker Compose file
COMPOSE_FILE := docker-compose.yml

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m

.PHONY: help up down logs build test lint clean migrate status

##@ General

help: ## Display this help
	@echo "Flashbacks - Microservice Architecture"
	@echo "======================================"
	@echo ""
	@echo "Usage: make <command> [SERVICE=<service-name>]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Commands:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2 } /^##@/ { printf "\n${YELLOW}%s${NC}\n", substr($$0, 5) }' $(MAKEFILE_LIST)

status: ## Show status of all services
	@docker compose -f $(COMPOSE_FILE) ps

##@ Docker Compose

up: ## Start all services
	@echo "${GREEN}Starting all services...${NC}"
	@docker compose -f $(COMPOSE_FILE) up -d
	@echo "${GREEN}All services started. Open http://localhost:5180${NC}"

down: ## Stop all services
	@echo "${YELLOW}Stopping all services...${NC}"
	@docker compose -f $(COMPOSE_FILE) down

logs: ## Show logs (use SERVICE=xxx to filter)
	@docker compose -f $(COMPOSE_FILE) logs -f $(SERVICE)

build: ## Build all services
	@echo "${GREEN}Building all services...${NC}"
	@docker compose -f $(COMPOSE_FILE) build

rebuild: ## Rebuild and restart a specific service
	@echo "${GREEN}Rebuilding $(SERVICE)...${NC}"
	@docker compose -f $(COMPOSE_FILE) up -d --build $(SERVICE)

##@ Development

dev-api: ## Run api-service locally
	@echo "${GREEN}Starting api-service in dev mode...${NC}"
	@cd backend/api-service && go run ./cmd/server/

dev-webui: ## Run webui locally
	@echo "${GREEN}Starting webui in dev mode...${NC}"
	@cd webapp && npm run dev

dev-exif: ## Run exif locally
	@echo "${GREEN}Starting exif in dev mode...${NC}"
	@cd backend/exif && go run ./cmd/server/

##@ Testing

test: ## Run tests for a service
	@echo "${GREEN}Running tests for $(SERVICE)...${NC}"
ifeq ($(SERVICE),api-service)
	@cd backend/api-service && go test ./internal/application/... -count=1 -v
else ifeq ($(SERVICE),webapp)
	@cd webapp && npm test
else ifeq ($(SERVICE),exif)
	@cd backend/exif && go test ./... -count=1 -v
else ifeq ($(SERVICE),ocr)
	@cd backend/ocr && go test ./... -count=1 -v
else
	@echo "${RED}Unknown service: $(SERVICE)${NC}"
	@exit 1
endif

test-all: ## Run tests for all services
	@$(MAKE) test SERVICE=api-service
	@$(MAKE) test SERVICE=webapp
	@$(MAKE) test SERVICE=exif
	@$(MAKE) test SERVICE=ocr

##@ Linting

lint: ## Run linter for a service
	@echo "${GREEN}Linting $(SERVICE)...${NC}"
ifeq ($(SERVICE),api-service)
	@cd backend/api-service && go vet ./...
else ifeq ($(SERVICE),webapp)
	@cd webapp && npm run lint && npx tsc -b
else ifeq ($(SERVICE),exif)
	@cd backend/exif && go vet ./...
else ifeq ($(SERVICE),ocr)
	@cd backend/ocr && go vet ./...
else
	@echo "${RED}Unknown service: $(SERVICE)${NC}"
	@exit 1
endif

lint-all: ## Run linters for all services
	@$(MAKE) lint SERVICE=api-service
	@$(MAKE) lint SERVICE=webapp
	@$(MAKE) lint SERVICE=exif
	@$(MAKE) lint SERVICE=ocr

##@ Database

db-reset: ## Reset PostgreSQL database (WARNING: deletes all data)
	@echo "${RED}WARNING: This will delete all data!${NC}"
	@read -p "Are you sure? (y/N) " confirm; \
	if [ "$$confirm" = "y" ]; then \
		docker compose -f $(COMPOSE_FILE) down -v; \
		docker compose -f $(COMPOSE_FILE) up -d postgres; \
		echo "${GREEN}Database reset complete.${NC}"; \
	else \
		echo "Aborted."; \
	fi

db-migrate: ## Run database migrations for api-service
	@echo "${GREEN}Running database migrations...${NC}"
	@cd backend/api-service && go run ./cmd/server/ --migrate-only

##@ Cleanup

clean: ## Remove build artifacts and caches
	@echo "${YELLOW}Cleaning up...${NC}"
	@cd backend/api-service && rm -f image-toolkit coverage.out
	@cd webapp && rm -rf dist node_modules/.vite
	@cd backend/exif && rm -f exif
	@cd backend/ocr && rm -f ocr
	@echo "${GREEN}Cleanup complete.${NC}"

prune: ## Remove unused Docker resources
	@echo "${YELLOW}Pruning Docker resources...${NC}"
	@docker system prune -f

##@ Code Generation

generate-types: ## Generate TypeScript types from OpenAPI specs
	@echo "${GREEN}Generating TypeScript types...${NC}"
	@cd webapp && npx openapi-typescript ../docs/api-contracts/api-service.yaml -o src/types/api.ts

generate-go: ## Generate Go code from OpenAPI specs
	@echo "${GREEN}Generating Go code...${NC}"
	@cd backend/api-service && go generate ./...

##@ Documentation

docs: ## Serve API documentation locally
	@echo "${GREEN}Serving API documentation...${NC}"
	@echo "Open the following files in your browser:"
	@echo "  - docs/api-contracts/api-service.yaml"
	@echo "  - docs/api-contracts/exif.yaml"
	@echo "  - docs/api-contracts/ocr.yaml"
