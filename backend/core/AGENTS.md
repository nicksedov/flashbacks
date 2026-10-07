# API Service Agent Context

## Overview

This is the backend API service for the Flashbacks platform, a Go application providing image management, OCR, geolocation, and AI-powered search capabilities.

## Stack

- Go 1.25+
- Gin HTTP framework
- PostgreSQL 16+ with pgvector extension
- GORM ORM with auto-migration
- exiftool for EXIF metadata
- gocluster for geographic clustering

## Structure

```
api-service/
├── cmd/server/main.go          # Entry point, DI
└── internal/
    ├── application/            # Business logic
    │   ├── agent/              # AI conversation agent
    │   ├── auth/               # Auth, sessions, users
    │   ├── background/         # Background job manager
    │   ├── geo/                # Geolocation, clustering
    │   ├── imaging/            # Scanning, OCR, thumbnails
    │   └── thumbnail/          # Thumbnail generation
    ├── domain/                 # Models (auth.go, media.go, conversation.go)
    ├── infrastructure/         # External integrations
    │   ├── config/             # Configuration
    │   ├── database/           # PostgreSQL, migrations
    │   ├── exifclient/         # EXIF service HTTP client
    │   ├── geocoder/           # Nominatim geocoding
    │   ├── llm/                # LLM clients (OpenAI, Ollama)
    │   ├── mcpserver/          # MCP server tools
    │   └── ocr/                # OCR classifier client
    └── interfaces/             # API layer
        ├── dto/                # Request/response DTOs
        ├── handler/            # HTTP handlers (Gin)
        ├── i18n/               # Internationalization (en/ru)
        └── middleware/         # Auth, CORS, CSRF, language
```

## Architecture

- **Clean Architecture**: Dependencies point inward
- **Manual DI**: No framework, explicit in main.go
- **Async Operations**: Goroutines for background tasks
- **GORM Auto-migration**: Schema managed at startup

## Coding Standards

### Go

- PascalCase for exported identifiers
- camelCase for unexported identifiers
- Explicit error handling, no panics
- **No identifier redeclaration** in same scope
- i18n: Use `Msg*` constants, convert: `string(i18n.MsgX)`
- Validation: `i18n.CreateValidationError(i18n.ValidationError)`
- JSON tags must match frontend TypeScript names (camelCase)

### Internationalization

- Located in `internal/interfaces/i18n/`
- Supports English (`en`) and Russian (`ru`)
- Use `Msg*` constants for message keys
- Keep en/ru translations in sync

## Constraints

### DO NOT

- ❌ Use `any` without justification
- ❌ Redeclare Go identifiers in same scope
- ❌ Use arbitrary strings for i18n keys
- ❌ Mix `Msg*`/`Err*` (use `Msg*`)
- ❌ Skip `MessageKey` → `string` conversion

### MUST

- ✅ Handle all Go errors explicitly
- ✅ Follow clean architecture principles
- ✅ Keep i18n en/ru in sync
- ✅ Cover new Go code with unit-tests
- ✅ Run backend unit tests after every code change — `go test ./internal/application/... -count=1`
- ✅ Fix failing tests before committing — zero failures required
- ✅ Match JSON tags with frontend TypeScript property names

## Commands

```bash
go mod tidy                              # Install dependencies
go build -o api-service ./cmd/server/    # Build binary
go run ./cmd/server/                     # Dev: http://localhost:5170
go test ./internal/application/... -count=1  # Run unit tests (ALWAYS after changes)
go test ./internal/application/... -v    # Verbose test output
go test ./... -coverprofile=coverage.out # Coverage report
```

## Environment

```env
DB_HOST=localhost
DB_PORT=5432
DB_NAME=api_db
DB_USER=postgres
DB_PASSWORD=postgres
SERVER_HOST=0.0.0.0
SERVER_PORT=5170
CORS_ORIGINS=http://localhost:5180
BOOTSTRAP_LOGIN=admin
BOOTSTRAP_PASSWORD=admin
OCR_SERVICE_URL=http://localhost:5174
EXIF_SERVICE_URL=http://localhost:5172
```

## Key Features

1. **Duplicate Detection**: MD5 + file size comparison
2. **Async Scanning**: Background jobs with progress tracking
3. **Thumbnails**: WebP generation, disk-cached
4. **OCR Classification**: Integration with external OCR service
5. **EXIF GPS Extraction**: Via EXIF service
6. **Geoclustering**: Geographic point clustering
7. **Session Auth**: With CSRF protection and rate limiting
8. **i18n**: Custom system (en/ru), strict typing
9. **Background Jobs**: Sync, cleanup, OCR, tags
10. **AI Chat**: Conversational search with LLM

## API Endpoints

See OpenAPI spec: `docs/api-contracts/api-service.yaml`

Key routes:
- `/api/auth/*` - Authentication
- `/api/scan` - Image scanning
- `/api/gallery` - Image gallery
- `/api/ocr/*` - OCR operations
- `/api/chat/*` - AI chat
- `/api/media/*` - Media management

## Validation

```bash
go build ./...                           # Compile check
go test ./internal/application/... -count=1  # Unit tests (must pass with 0 failures)