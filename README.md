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

### Prerequisites
- Docker & Docker Compose
- Go 1.25+ (for local development)
- Node.js 22+ (for frontend development)

### Run All Services
```bash
git clone https://github.com/flashbacks/flashbacks.git
cd flashbacks
docker compose up -d
```

Open http://localhost:5180 in your browser.

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
