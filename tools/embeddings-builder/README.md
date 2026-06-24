# Embeddings Builder

CLI utility for populating the `tag_embeddings` table from existing `image_tags` records.

## Overview

This tool processes images that have AI-generated tags and creates vector embeddings for semantic search. It is idempotent by design: it tracks tag content changes via MD5 hashes and skips images whose embeddings already exist and whose tag content has not changed.

## Stack

- **Go 1.25+** - Runtime
- **PostgreSQL 16+** with pgvector - Database
- **GORM** - ORM
- **Ollama / OpenAI** - Embedding providers

## Usage

```bash
# Count images needing embeddings (dry run)
go run ./cmd/embeddings-builder --dry-run

# Process all with default batch size (100)
go run ./cmd/embeddings-builder

# Smaller batches if API timeouts occur
go run ./cmd/embeddings-builder --batch-size 50

# Recompute all embeddings regardless of existing data
go run ./cmd/embeddings-builder --force

# Build binary
go build -o embeddings-builder ./cmd/embeddings-builder/
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--batch-size` | Number of images per embedding API call | `100` |
| `--dry-run` | Count images needing embeddings without processing | `false` |
| `--force` | Recompute all embeddings, ignoring existing data and tag hashes | `false` |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_NAME` | Database name | `api_db` |
| `DB_USER` | Database user | `postgres` |
| `DB_PASSWORD` | Database password | `postgres` |

## How It Works

1. Connects to the PostgreSQL database used by `api-service`
2. Reads `llm_settings` and `llm_providers` tables to determine the embedding provider and model
3. Creates per-model child tables (`tag_embeddings_<model_name>`) with the correct vector dimension
4. Processes images in batches, fetching their tags and generating embeddings
5. Stores embeddings in the child table and tracks tag hashes for idempotency

## Project Structure

```
embeddings-builder/
├── cmd/embeddings-builder/
│   ├── main.go         # Entry point, batch processing logic
│   ├── db.go           # Database connection
│   ├── embedding.go    # Embedding client (Ollama, OpenAI)
│   └── models.go       # GORM models
├── go.mod
└── README.md
```

## License

MIT
