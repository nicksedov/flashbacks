# Flashbacks API Service

Backend service for the Flashbacks image management platform.

## Stack

- **Go 1.25+** - Runtime
- **Gin** - HTTP framework
- **PostgreSQL 16+** with pgvector - Database
- **GORM** - ORM with auto-migration
- **exiftool** - EXIF metadata extraction
- **gocluster** - Geographic clustering

### Run Locally

```bash
# Install dependencies
go mod download

# Run the service
go run ./cmd/server/
```

### Run with Docker

```bash
# Build the image
docker build -t api-service .

# Run the container
docker run -p 5170:5170 \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=postgres \
  -e DB_NAME=image_toolkit \
  api-service
```

## Development

The API service runs on `http://localhost:5170` by default.

### Project Structure

```
api-service/
├── cmd/server/main.go          # Entry point, dependency injection
├── internal/
│   ├── application/            # Business logic
│   │   ├── agent/              # AI conversation agent
│   │   ├── auth/               # Authentication & sessions
│   │   ├── background/         # Background job manager
│   │   ├── geo/                # Geolocation & clustering
│   │   ├── imaging/            # Image scanning, OCR, thumbnails
│   │   └── thumbnail/          # Thumbnail generation
│   ├── domain/                 # Domain models
│   ├── infrastructure/         # External integrations
│   │   ├── config/             # Configuration
│   │   ├── database/           # Database layer
│   │   ├── exifclient/         # EXIF service client
│   │   ├── geocoder/           # Geocoding service
│   │   ├── llm/                # LLM clients (OpenAI, Ollama)
│   │   ├── mcpserver/          # MCP server implementation
│   │   └── ocr/                # OCR client
│   └── interfaces/             # API layer
│       ├── dto/                # Data transfer objects
│       ├── handler/            # HTTP handlers
│       ├── i18n/               # Internationalization
│       └── middleware/         # HTTP middleware
├── go.mod
└── go.sum
```

## Testing

```bash
# Run all tests
go test ./internal/application/... -count=1

# Verbose output
go test ./internal/application/... -v

# Coverage report
go test ./... -coverprofile=coverage.out
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_NAME` | Database name | `api_db` |
| `DB_USER` | Database user | `postgres` |
| `DB_PASSWORD` | Database password | - |
| `SERVER_HOST` | Server bind address | `0.0.0.0` |
| `SERVER_PORT` | Server port | `5170` |
| `CORS_ORIGINS` | Allowed CORS origins | `*` |
| `BOOTSTRAP_LOGIN` | Initial admin login | - |
| `BOOTSTRAP_PASSWORD` | Initial admin password | - |
| `OCR_SERVICE_URL` | OCR service URL | - |
| `EXIF_SERVICE_URL` | EXIF service URL | - |

## API Documentation

OpenAPI specification: [`docs/api-contracts/api-service.yaml`](../docs/api-contracts/api-service.yaml)

## Architecture

- **Clean Architecture** with manual dependency injection
- **Async operations** using goroutines for background tasks
- **GORM auto-migration** for schema management
- **Session-based authentication** with CSRF protection

## License

MIT