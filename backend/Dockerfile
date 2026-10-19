# syntax=docker/dockerfile:1
#
# Unified Dockerfile for Flashbacks backend microservices
#
# Build specific service:
#   docker build --target=core        -t flashbacks/core .
#   docker build --target=exif        -t flashbacks/exif .
#   docker build --target=ocr         -t flashbacks/ocr .
#   docker build --target=registry    -t flashbacks/registry .
#
# Build context must be the backend/ directory (parent of this Dockerfile).

# ======================================================================
# Common builder stage — builds every service binary
# ======================================================================
FROM golang:1.27.1-alpine AS builder

WORKDIR /build

# Install all build dependencies (CGO deps needed for gosseract/ocr)
# core and exif set CGO_ENABLED=0, so extra packages are harmless
RUN apk update && apk add --no-cache \
    git \
    gcc \
    g++ \
    tesseract-ocr-dev \
    leptonica-dev

# ---------- Copy all module manifests first ----------
# core, exif and ocr all `replace ../shared` and `replace ../registry`, so every
# replaced module directory must contain its go.mod before any `go mod download`
# runs. Copying all manifests up front keeps the dependency layers cacheable.
# shared/ is copied in full because it must be at /build/shared/ for the replace.
COPY shared/ /build/shared/
COPY registry/go.mod registry/go.sum ./registry/
COPY core/go.mod core/go.sum ./core/
COPY exif/go.mod exif/go.sum ./exif/
COPY ocr/go.mod ocr/go.sum ./ocr/

# ---------- Download dependencies per module ----------
RUN cd core && go mod download
RUN cd exif && go mod download
RUN cd ocr && go mod download
RUN cd registry && go mod download

# ---------- Copy all source code ----------
COPY core/ ./core/
COPY exif/ ./exif/
COPY ocr/ ./ocr/
COPY registry/ ./registry/

# ---------- Build all binaries ----------
RUN cd core && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /build/bin/core ./cmd/server
RUN cd exif && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /build/bin/exif ./cmd/server
RUN cd ocr  && CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /build/bin/ocr ./cmd/server
RUN cd registry && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /build/bin/registry ./cmd/server

# ======================================================================
# Runtime stages — each pulls only its binary from the common builder
# ======================================================================

# --- core runtime ---
FROM alpine:3.24 AS core

ARG SERVER_PORT=5170

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /build/bin/core ./core

ENV SERVER_PORT=${SERVER_PORT}
EXPOSE ${SERVER_PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -q --spider "http://localhost:${SERVER_PORT}/api/auth/status" || exit 1

CMD ["./core"]

# --- exif runtime ---
FROM alpine:3.24 AS exif

ARG EXIF_PORT=5172

RUN apk add --no-cache perl exiftool ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/bin/exif ./exif

ENV EXIF_PORT=${EXIF_PORT}
EXPOSE ${EXIF_PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider "http://localhost:${EXIF_PORT}/exif/health" || exit 1

CMD ["./exif"]

# --- ocr runtime ---
FROM alpine:3.24 AS ocr

ARG OCR_PORT=5174

RUN apk update && apk add --no-cache \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-rus \
    libstdc++ \
    ca-certificates

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/bin/ocr ./ocr

RUN chown -R appuser:appgroup /app

USER appuser

ENV OCR_PORT=${OCR_PORT}
EXPOSE ${OCR_PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost:${OCR_PORT}/ocr/api/health || exit 1

CMD ["./ocr"]

# --- registry runtime ---
FROM alpine:3.24 AS registry

ARG REGISTRY_HTTP_PORT=5176

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /build/bin/registry ./registry

ENV REGISTRY_HTTP_ADDR=:${REGISTRY_HTTP_PORT} \
    REGISTRY_DATA_DIR=/data
EXPOSE ${REGISTRY_HTTP_PORT}
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider "http://localhost:${REGISTRY_HTTP_PORT}/registry/v1/health" || exit 1

CMD ["./registry"]
