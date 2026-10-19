# Flashbacks Architecture

## Overview

Flashbacks is a microservices-based image management platform with AI-powered search capabilities. The system consists of independent services communicating via REST APIs.

## Services

| Service | Port | Description | Technology |
|---------|------|-------------|------------|
| **webui** | 5180 | Frontend SPA | React 19, TypeScript 6, Vite 8 |
| **core** | 5170 | Main backend API | Go 1.25+, Gin, PostgreSQL 16+ |
| **exif** | 5172 | EXIF metadata extraction | Go 1.25+ |
| **ocr** | 5174 | Text recognition in images | Go 1.25+ |
| **registry** | 5176 | Service discovery + configuration KV store | Go 1.25+, embedded etcd |
| **postgres** | 5432 | Database (multiple logical DBs) | PostgreSQL 16 with pgvector |

## Project Structure

```
flashbacks/
├── webapp/                   # React 19 SPA frontend
├── backend/                  # Backend microservices
│   ├── core/          # Core Go API service
│   ├── exif/                 # EXIF metadata service
│   ├── ocr/                  # OCR text detection service
│   └── registry/             # Service discovery + config registry (embedded etcd)
├── tools/                    # Utilities (embeddings-builder)
├── docs/                     # Shared documentation
│   ├── api-contracts/        # API contracts (OpenAPI specs)
│   └── mcp-contracts/        # MCP tool contracts
├── scripts/                  # Host-native dev scripts (dev.sh, bootstrap.sh)
├── tests/                    # Integration tests (pytest for OCR)
├── skills/                   # Shared AI agent skills
└── Makefile                  # Common commands
```

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Client (Browser)                            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              webui :5180                                 │
│                    React 19 SPA with Vite dev server                     │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ /api/* (proxied)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                               core :5170                                │
│                         Main API Gateway Service                         │
│                                                                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Auth      │  │   Gallery   │  │   OCR       │  │   Chat      │    │
│  │   Module    │  │   Module    │  │   Module    │  │   Module    │    │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│                                                                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                      │
│  │   Scanner   │  │ Thumbnail   │  │   Geo       │                      │
│  │   Module    │  │   Module    │  │   Module    │                      │
│  └─────────────┘  └─────────────┘  └─────────────┘                      │
└─────────────────────────────────────────────────────────────────────────┘
        │                   │                   │
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ exif          │   │ocr            │   │   postgres    │
│    :5172      │   │    :5174      │   │    :5432      │
│               │   │               │   │               │
│ EXIF metadata │   │ Text recognition│  │  flashbacks   │
│ extraction    │   │ & classification│ │  (pgvector)   │
└───────────────┘   └───────────────┘   └───────────────┘
```

All Go services (`core`, `exif`, `ocr`) register with the **registry** (`:5176`,
embedded etcd) and read their configuration, secrets, and peer locations from it.

## Data Flow

### Image Scanning

1. User triggers scan via **webui**
2. **core** scans directories for new images
3. For each image:
   - Calls **exif** to extract metadata
   - Calls **ocr** to detect text
   - Generates thumbnail
   - Stores results in **postgres** (shared `flashbacks` database)

### AI Chat / Smart Search

1. User sends query via **webui** chat interface
2. **core** processes query through LLM agent
3. Agent may:
   - Search images via tag embeddings (pgvector similarity)
   - Call OCR service for text-based queries
   - Return results with image previews

### Authentication Flow

1. User submits credentials to **core** `/api/auth/login`
2. Service creates session and returns session cookie
3. Subsequent requests include session cookie
4. CSRF token required for state-changing operations

## Database Schema

A single PostgreSQL database (`flashbacks`) is shared by every service. `core`
owns most tables and auto-migrates them on startup; the `exif` service owns
`image_metadata` and `geolocation_caches`.

```
# Owned by core
image_files           # Image metadata
image_tags            # AI-generated tags
tag_embeddings        # Parent table for embeddings
tag_embeddings_*      # Per-model child tables (pgvector)
users                 # User accounts
sessions              # Authentication sessions
llm_settings          # LLM provider configuration
llm_providers         # LLM provider instances
conversations         # Chat history
conversation_messages # Chat messages

# Owned by exif
image_metadata        # EXIF metadata per image
geolocation_caches    # Geolocation data
```

## Security

- **Session-based authentication** with bcrypt password hashing
- **CSRF protection** for all state-changing requests
- **Rate limiting** on login endpoint
- **CORS** configured per environment
- **API keys** stored encrypted in database

## Inter-Service Communication

All services communicate via HTTP REST:

| From | To | Purpose |
|------|-----|---------|
| webui | core | All user operations |
| core/exif/ocr | registry | Registration, configuration, secrets |
| core | exif | EXIF extraction (URL resolved from discovery) |
| core | ocr | Text recognition (URL resolved from discovery) |
| embeddings-builder | core DB | Populate embeddings |

## Configuration Management

The **registry** service (`backend/registry`, port `5176`) is the single source
of truth for service configuration and discovery. It embeds a single-node etcd
instance and exposes a REST facade:

- `/registry/v1/config/<service>/<key>` — plaintext configuration values
- `/registry/v1/secrets/<service>/<key>` — AES-256-GCM encrypted secrets
- `/registry/v1/services/<service>/<instance>` — TTL-leased registrations

Each service requires only bootstrap environment variables: `REGISTRY_URL`,
`REGISTRY_TOKEN` (optional), `SERVICE_NAME`, `SERVICE_ID`, and `SERVICE_HOST`.
All other values are read from the registry with an
env-override > registry > built-in-default precedence. When `REGISTRY_URL` is
empty, a service falls back to its local environment and defaults, so a registry
outage degrades rather than blocks startup.

For local development a single repository-level `.env` (created from
`.env.example`) is loaded by the Makefile and `scripts/dev.sh` and exported to
every service. It now carries the registry bootstrap variables plus fallback
values.

The HTTP integration suites in `tests/` do not read `.env`. They load their own
git-ignored `test.env` profile (created from `.env.example` the same way) and
take the database from `DB_NAME`, defaulting to the throwaway `flashbacks-test`
database.

## Local Development

Services run directly on the host inside the IDE container; PostgreSQL is an
external, shared service configured through `.env` (see `.env.example`).

```bash
make env      # create .env, then set the DB credentials
make setup    # install host prerequisites (Go deps, npm, pytest venv, exiftool)
make up       # start registry first, then core, exif, ocr and webapp
make status   # show PIDs and URLs
make down     # stop everything
```

Run a single service in the foreground:

```bash
make dev-registry # registry :5176
make dev-core    # core   :5170
make dev-exif    # exif   :5172
make dev-ocr     # ocr    :5174
make dev-webapp  # webapp :5180
```

Embeddings Builder (on-demand):

```bash
cd tools/embeddings-builder && go run ./cmd/embeddings-builder
```

## Deployment

Each service is deployed independently:

1. **webui**: Static assets served by nginx or CDN
2. **registry**: Containerized embedded etcd + REST facade on 5176, internal network only
3. **core**: Containerized, exposed on port 5170
4. **exif**: Containerized, internal network only
5. **ocr**: Containerized, internal network only
6. **postgres**: External PostgreSQL 17 with pgvector (shared by all services)

## Future Enhancements

- Event-driven architecture with message queue (RabbitMQ/NATS)
- Distributed tracing (Jaeger/OpenTelemetry)
- GraphQL API for complex queries
- Caching layer (Redis) for frequent queries
- Horizontal scaling of core
