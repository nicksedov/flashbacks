# Flashbacks

A microservice-based application for finding and managing duplicate images.

## Services

| Service | Repository | Description | Port |
|---|---|---|---|
| **webapp** | [flashbacks/webapp](https://github.com/flashbacks/webapp) | React SPA frontend | 5180 |
| **core** | [flashbacks/core-service](https://github.com/flashbacks/core) | Core API (auth, scan, gallery, OCR, tags) | 5170 |
| **exif** | [flashbacks/exif](https://github.com/flashbacks/exif) | EXIF metadata extraction & GPS writing | 5172 |
| **ocr** | [flashbacks/ocr](https://github.com/flashbacks/ocr) | OCR text detection (Tesseract) | 5174 |

## Quick Start

The development stack runs directly on the host inside the IDE container — no
Docker images and no docker-compose. PostgreSQL (with the `pgvector` extension)
is an external, shared service configured through `.env`.

### Prerequisites
- Go 1.25+
- Node.js 22+ and npm
- Python 3.11+ (for the OCR pytest suite)
- Tesseract OCR with development headers (for the `ocr` service)
- Access to a PostgreSQL 16+ instance with the `pgvector` extension

### Setup
```bash
git clone https://github.com/flashbacks/flashbacks.git
cd flashbacks
make env      # create .env, then set the DB_* credentials
make setup    # Go modules, npm deps, pytest venv, local ExifTool
make doctor   # verify the toolchain and database reachability
```

### Run All Services
```bash
make up       # start core (5170), exif (5172), ocr (5174), webapp (5180)
make status   # show PIDs and URLs
make logs     # tail service logs
make down     # stop everything
```

Open http://localhost:5180 in your browser. To run a single service in the
foreground, use `make dev-core`, `make dev-exif`, `make dev-ocr` or
`make dev-webapp`.

### Testing
```bash
make test-all               # Go (core/exif/ocr) + webapp unit tests
make test-ocr-integration   # pytest against a running OCR service
make test-webapp-integration # webapp HTTP integration tests (tests/webapp) against the test DB
```

### Development Setup
See individual service READMEs:
- [webapp Development Guide](webapp/README.md)
- [core Development Guide](backend/core/README.md)
- [exif Development Guide](backend/exif/README.md)
- [ocr Development Guide](backend/ocr/README.md)
- [tools Development Guide](tools/README.md)

## Architecture
See [docs/architecture.md](docs/architecture.md).

## API Documentation
- [core OpenAPI](docs/api-contracts/core.yaml)
- [exif OpenAPI](docs/api-contracts/exif.yaml)
- [ocr OpenAPI](docs/api-contracts/ocr.yaml)

## Migration from Monorepo
This project was migrated from the `image-toolkit` monorepo to a microservice architecture.
See [MIGRATION_PLAN.md](MIGRATION_PLAN.md) for details.

## License
MIT
