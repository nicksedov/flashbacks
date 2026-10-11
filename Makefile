# Flashbacks — host-native development Makefile
# ==============================================
# Runs every service directly inside the IDE container (no Docker, no
# docker-compose). PostgreSQL is an external, shared service described in .env.
#
# Quick start:
#   make env          # create .env from .env.example (then set DB credentials)
#   make setup        # install all host prerequisites (Go deps, npm, venv, exiftool)
#   make doctor       # verify toolchain + database reachability
#   make up           # start core, exif, ocr and webapp in the background
#
# Service-specific commands accept SERVICE=<core|exif|ocr|webapp>:
#   make test SERVICE=core
#   make lint SERVICE=webapp

SHELL := /bin/bash
.DEFAULT_GOAL := help

# The container ships an unconfigured en_US.UTF-8 locale; pin a working one so
# recipe shells do not emit spurious "cannot change locale" warnings.
export LC_ALL := C

# ─── Configuration ───────────────────────────────────────────────────────────
SERVICE   ?= core
ENVFILE   := .env
DEV       := scripts/dev.sh
BOOTSTRAP := scripts/bootstrap.sh

# Python virtualenv used by the OCR integration (pytest) suite
PYTEST_VENV := tests/ocr/integration/.venv
PY          := $(PYTEST_VENV)/bin/python

# Load .env into the current shell (prefix for recipes that need DB creds)
LOAD_ENV = set -a; [ -f $(ENVFILE) ] && . ./$(ENVFILE); set +a;

# Colours for output
GREEN  := \033[0;32m
YELLOW := \033[1;33m
RED    := \033[0;31m
NC     := \033[0m

.PHONY: help all doctor env setup setup-env setup-webapp setup-pytest deps tools \
        build build-core build-exif build-ocr \
        up start down stop restart status logs \
        run-core run-exif run-ocr run-webapp \
        dev-core dev-exif dev-ocr dev-webapp \
        test test-all test-go test-core test-exif test-ocr test-webapp test-ocr-integration test-pytest \
        lint lint-all lint-core lint-exif lint-ocr lint-webapp \
        db-migrate db-shell generate-types generate-go clean clean-tools

##@ General

help: ## Display this help
	@echo "Flashbacks — host-native development"
	@echo "===================================="
	@echo ""
	@echo "Usage: make <command> [SERVICE=<core|exif|ocr|webapp>]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Commands:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  ${GREEN}%-22s${NC} %s\n", $$1, $$2 } \
	     /^##@/ { printf "\n${YELLOW}%s${NC}\n", substr($$0, 5) }' $(MAKEFILE_LIST)

doctor: ## Check host toolchain, local tools and database reachability
	@bash -euo pipefail -c ' \
	  check() { printf "  %-12s" "$$1"; if command -v "$$1" >/dev/null 2>&1; then echo "OK"; else echo "MISSING"; fi; }; \
	  echo "Toolchain:"; \
	  for c in go node npm python3 tesseract perl curl tar setsid; do check "$$c"; done; \
	  printf "  %-12s" "exiftool"; \
	  if command -v exiftool >/dev/null 2>&1 || [ -x .tools/bin/exiftool ]; then echo "OK (host or .tools)"; else echo "MISSING (run: make tools)"; fi'
	@bash -euo pipefail -c ' \
	  set -a; [ -f $(ENVFILE) ] && . ./$(ENVFILE); set +a; \
	  host="$${DB_HOST:-postgreshost}"; port="$${DB_PORT:-5432}"; \
	  printf "  %-12s" "PostgreSQL"; \
	  if timeout 3 bash -c "exec 3<>/dev/tcp/$$host/$$port" 2>/dev/null; then \
	    echo "reachable ($$host:$$port)"; else echo "UNREACHABLE ($$host:$$port)"; fi'

env: ## Create .env from .env.example (does not overwrite an existing .env)
	@if [ -f $(ENVFILE) ]; then \
	  echo "${YELLOW}$(ENVFILE) already exists — leaving it untouched.${NC}"; \
	else \
	  cp .env.example $(ENVFILE); \
	  echo "${GREEN}Created $(ENVFILE). Set DB credentials before starting services.${NC}"; \
	fi

##@ Setup

setup: ## Install every host prerequisite (Go deps, npm, pytest venv, exiftool)
	@$(BOOTSTRAP) all

setup-env: env ## Alias for 'env'

setup-webapp: ## Install webapp npm dependencies
	@$(BOOTSTRAP) webapp

setup-pytest: ## Create the pytest virtualenv and install its requirements
	@$(BOOTSTRAP) pytest

deps: ## Download Go module dependencies for every service
	@$(BOOTSTRAP) godeps

tools: ## Install ExifTool locally into .tools/ (no root required)
	@$(BOOTSTRAP) exiftool

##@ Build

build: ## Compile core, exif and ocr binaries into .run/bin
	@$(DEV) build core exif ocr

build-core: ## Compile the core binary
	@$(DEV) build core

build-exif: ## Compile the exif binary
	@$(DEV) build exif

build-ocr: ## Compile the ocr binary
	@$(DEV) build ocr

##@ Run (background)

up: ## Start all services in the background
	@$(DEV) start all

start: up ## Alias for 'up'

down: ## Stop all background services
	@$(DEV) stop all

stop: down ## Alias for 'down'

restart: ## Restart all background services
	@$(DEV) restart all

status: ## Show status of all services
	@$(DEV) status all

logs: ## Tail logs for all services (SERVICE=<name> to filter one)
	@$(DEV) logs $(if $(filter command line,$(origin SERVICE)),$(SERVICE),all)

run-core: ## Start core in the background
	@$(DEV) start core

run-exif: ## Start exif in the background
	@$(DEV) start exif

run-ocr: ## Start ocr in the background
	@$(DEV) start ocr

run-webapp: ## Start the webapp dev server in the background
	@$(DEV) start webapp

##@ Run (foreground)

dev-core: ## Run core in the foreground
	@$(DEV) run core

dev-exif: ## Run exif in the foreground
	@$(DEV) run exif

dev-ocr: ## Run ocr in the foreground
	@$(DEV) run ocr

dev-webapp: ## Run the webapp dev server in the foreground
	@$(DEV) run webapp

##@ Testing

test: ## Run unit tests (SERVICE=<core|exif|ocr|webapp|ocr-integration>)
ifeq ($(SERVICE),core)
	@$(MAKE) --no-print-directory test-core
else ifeq ($(SERVICE),exif)
	@$(MAKE) --no-print-directory test-exif
else ifeq ($(SERVICE),ocr)
	@$(MAKE) --no-print-directory test-ocr
else ifeq ($(SERVICE),webapp)
	@$(MAKE) --no-print-directory test-webapp
else ifeq ($(SERVICE),ocr-integration)
	@$(MAKE) --no-print-directory test-ocr-integration
else
	@echo "${RED}Unknown service: $(SERVICE)${NC}"; exit 1
endif

test-core: ## Run core Go unit tests
	@echo "${GREEN}Testing core…${NC}"
	@cd backend/core && go test ./internal/application/... -count=1

test-exif: ## Run exif Go unit tests
	@echo "${GREEN}Testing exif…${NC}"
	@cd backend/exif && go test ./... -count=1

test-ocr: ## Run ocr Go unit tests
	@echo "${GREEN}Testing ocr…${NC}"
	@cd backend/ocr && go test ./... -count=1

test-webapp: ## Run webapp unit tests (Vitest)
	@echo "${GREEN}Testing webapp…${NC}"
	@cd webapp && npm test

test-go: ## Run all Go unit tests
	@$(MAKE) --no-print-directory test-core
	@$(MAKE) --no-print-directory test-exif
	@$(MAKE) --no-print-directory test-ocr

test-ocr-integration: ## Run OCR HTTP integration tests (pytest) against the running service
	@if [ ! -x "$(PY)" ]; then \
	  echo "${RED}pytest venv missing — run: make setup-pytest${NC}"; exit 1; \
	fi
	@$(DEV) status ocr | grep -q running || { echo "${YELLOW}Starting OCR service…${NC}"; $(DEV) start ocr; }
	@$(LOAD_ENV) $(PY) -m pytest tests/ocr/integration \
	  --base-url "$${OCR_CLASSIFIER_URL:-http://localhost:5174}" -v

test-pytest: test-ocr-integration ## Alias for 'test-ocr-integration'

test-all: ## Run every unit test suite
	@$(MAKE) --no-print-directory test-go
	@$(MAKE) --no-print-directory test-webapp

##@ Linting

lint: ## Lint a service (SERVICE=<core|exif|ocr|webapp>)
ifeq ($(SERVICE),core)
	@$(MAKE) --no-print-directory lint-core
else ifeq ($(SERVICE),exif)
	@$(MAKE) --no-print-directory lint-exif
else ifeq ($(SERVICE),ocr)
	@$(MAKE) --no-print-directory lint-ocr
else ifeq ($(SERVICE),webapp)
	@$(MAKE) --no-print-directory lint-webapp
else
	@echo "${RED}Unknown service: $(SERVICE)${NC}"; exit 1
endif

lint-core: ## Vet core
	@cd backend/core && go vet ./...

lint-exif: ## Vet exif
	@cd backend/exif && go vet ./...

lint-ocr: ## Vet ocr
	@cd backend/ocr && go vet ./...

lint-webapp: ## Lint + type-check webapp
	@cd webapp && npm run lint && npx tsc -b

lint-all: ## Lint every service
	@$(MAKE) --no-print-directory lint-core
	@$(MAKE) --no-print-directory lint-exif
	@$(MAKE) --no-print-directory lint-ocr
	@$(MAKE) --no-print-directory lint-webapp

##@ Database

db-migrate: ## Apply the core schema (starts core, waits for readiness, stops it)
	@echo "${GREEN}Applying database migrations via core…${NC}"
	@$(DEV) start core
	@$(DEV) stop core

db-shell: ## Open psql in the external Postgres container
	@$(LOAD_ENV) docker exec -it postgres psql -U "$${DB_USER}" -d "$${DB_NAME}"

##@ Code Generation

generate-types: ## Generate TypeScript types from the core OpenAPI spec
	@echo "${GREEN}Generating TypeScript types…${NC}"
	@cd webapp && npx openapi-typescript ../docs/api-contracts/core.yaml -o src/types/api.ts

generate-go: ## Regenerate Go code (Wire DI) for core
	@echo "${GREEN}Generating Go code…${NC}"
	@cd backend/core && go generate ./...

##@ Cleanup

clean: ## Remove build artifacts, runtime state and caches
	@echo "${YELLOW}Cleaning up…${NC}"
	@rm -rf .run
	@rm -rf webapp/dist
	@cd backend/core && rm -f coverage.out
	@echo "${GREEN}Cleanup complete.${NC}"

clean-tools: ## Remove locally bootstrapped tools (.tools/) and the pytest venv
	@echo "${YELLOW}Removing .tools/ and the pytest virtualenv…${NC}"
	@rm -rf .tools tests/ocr/integration/.venv
	@echo "${GREEN}Done.${NC}"
