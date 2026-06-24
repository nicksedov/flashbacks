# Flashbacks Architecture

## Overview

Flashbacks is a microservices-based image management platform with AI-powered search capabilities. The system consists of independent services communicating via REST APIs.

## Services

| Service | Port | Description | Technology |
|---------|------|-------------|------------|
| **webui** | 5180 | Frontend SPA | React 19, TypeScript 6, Vite 8 |
| **api-service** | 5170 | Main backend API | Go 1.25+, Gin, PostgreSQL 16+ |
| **exif** | 5172 | EXIF metadata extraction | Go 1.25+ |
| **ocr** | 5174 | Text recognition in images | Go 1.25+ |
| **postgres** | 5432 | Database (multiple logical DBs) | PostgreSQL 16 with pgvector |

## Project Structure

```
flashbacks/
├── webapp/                   # React 19 SPA frontend
├── backend/                  # Backend microservices
│   ├── api-service/          # Core Go API service
│   ├── exif/                 # EXIF metadata service
│   └── ocr/                  # OCR text detection service
├── tools/                    # Utilities (embeddings-builder)
├── docs/                     # Shared documentation
│   ├── api-contracts/        # API contracts (OpenAPI specs)
│   └── mcp-contracts/        # MCP tool contracts
├── skills/                   # Shared AI agent skills
├── docker-compose.yml        # Local dev environment
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
│                           api-service :5170                              │
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
│ EXIF metadata │   │ Text recognition│  │  api_db       │
│ extraction    │   │ & classification│ │  exif_db      │
└───────────────┘   └───────────────┘   └───────────────┘
```

## Data Flow

### Image Scanning

1. User triggers scan via **webui**
2. **api-service** scans directories for new images
3. For each image:
   - Calls **exif** to extract metadata
   - Calls **ocr** to detect text
   - Generates thumbnail
   - Stores results in **postgres** (`api_db`)

### AI Chat / Smart Search

1. User sends query via **webui** chat interface
2. **api-service** processes query through LLM agent
3. Agent may:
   - Search images via tag embeddings (pgvector similarity)
   - Call OCR service for text-based queries
   - Return results with image previews

### Authentication Flow

1. User submits credentials to **api-service** `/api/auth/login`
2. Service creates session and returns session cookie
3. Subsequent requests include session cookie
4. CSRF token required for state-changing operations

## Database Schema

### api_db (Primary Database)

```
image_files          # Image metadata
image_tags           # AI-generated tags
tag_embeddings       # Parent table for embeddings
tag_embeddings_*     # Per-model child tables (pgvector)
users                # User accounts
sessions             # Authentication sessions
llm_settings         # LLM provider configuration
llm_providers        # LLM provider instances
conversations        # Chat history
messages             # Chat messages
```

### exif_db (EXIF Metadata)

```
exif_data            # EXIF metadata per image
gps_coordinates      # Geolocation data
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
| webui | api-service | All user operations |
| api-service | exif | EXIF extraction |
| api-service | ocr | Text recognition |
| embeddings-builder | api-service DB | Populate embeddings |

## Configuration Management

Each service uses environment variables (12-factor app):

```
# api-service/.env.example
DB_HOST=localhost
DB_PORT=5432
DB_NAME=api_db
DB_USER=postgres
DB_PASSWORD=postgres
SERVER_PORT=5170
CORS_ORIGINS=http://localhost:5180
BOOTSTRAP_LOGIN=admin
BOOTSTRAP_PASSWORD=admin
OCR_SERVICE_URL=http://localhost:5174
EXIF_SERVICE_URL=http://localhost:5172
```

## Local Development

```bash
# Start all services via Docker Compose
docker-compose up -d

# Or run services individually:

# Frontend
cd webui && npm run dev

# API Service
cd api-service && go run ./cmd/server/

# Embeddings Builder (on-demand)
cd tools/embeddings-builder && go run ./cmd/embeddings-builder
```

## Deployment

Each service is deployed independently:

1. **webui**: Static assets served by nginx or CDN
2. **api-service**: Containerized, exposed on port 5170
3. **exif**: Containerized, internal network only
4. **ocr**: Containerized, internal network only
5. **postgres**: Managed database service or containerized

## Future Enhancements

- Event-driven architecture with message queue (RabbitMQ/NATS)
- Distributed tracing (Jaeger/OpenTelemetry)
- GraphQL API for complex queries
- Caching layer (Redis) for frequent queries
- Horizontal scaling of api-service
