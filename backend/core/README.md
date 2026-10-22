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
docker build -t core .

# Run the container
docker run -p 5170:5170 \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_USER=flashbacks \
  -e DB_PASSWORD=postgres \
  -e DB_NAME=flashbacks \
  core
```

## Development

The API service runs on `http://localhost:5170` by default.

### Project Structure

```
core/
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
| `ACCOUNT_CREATION_MODE` | Account creation mode: `admin_only`, `self_service`, `self_service_with_approval` | `admin_only` |
| `OCR_SERVICE_URL` | OCR service URL | - |
| `EXIF_SERVICE_URL` | EXIF service URL | - |

## Thumbnail Cache

Thumbnails are generated on demand and cached on disk (WebP by default). Before
resizing, generation applies the EXIF `Orientation` transform read locally from
the source file, using the same mapping the browser applies when it displays the
original: `6` rotates 90° clockwise, `8` rotates 270° clockwise, `3` rotates 180°,
and the mirrored values `2`, `4`, `5`, `7` are flipped as well. Note that
`imaging.Rotate90`/`Rotate270` rotate counter-clockwise, so those helpers must be
paired with the values in reverse.

On startup the service runs a one-time orientation backfill: it scans indexed
image files and regenerates cached thumbnails for photos whose orientation is not
`1`. Completion is recorded as `.orientation-backfill.v2` in the cache directory;
delete that file, or bump the marker name, to force another pass (the marker is
bumped whenever the rotation mapping changes, since only a new marker name
re-processes an already corrected cache).

## API Documentation

OpenAPI specification: [`docs/api-contracts/core.yaml`](../docs/api-contracts/core.yaml)

## Architecture

- **Clean Architecture** with manual dependency injection
- **Async operations** using goroutines for background tasks
- **GORM auto-migration** for schema management
- **Session-based authentication** with CSRF protection

## License

MIT