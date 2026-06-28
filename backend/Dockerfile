# syntax=docker/dockerfile:1
#
# Unified Dockerfile for Flashbacks backend microservices
#
# Build specific service:
#   docker build --target=api-service -t flashbacks/api-service .
#   docker build --target=exif        -t flashbacks/exif .
#   docker build --target=ocr         -t flashbacks/ocr .
#
# Build context must be the backend/ directory (parent of this Dockerfile).

# ======================================================================
# Common builder stage — builds all three binaries
# ======================================================================
FROM golang:1.26.4-alpine AS builder

WORKDIR /build

# Install all build dependencies (CGO deps needed for gosseract/ocr)
# api-service and exif set CGO_ENABLED=0, so extra packages are harmless
RUN apk update && apk add --no-cache \
    git \
    gcc \
    g++ \
    tesseract-ocr-dev \
    leptonica-dev

# ---------- Copy shared module ----------
# Must be at /build/shared/ so that `replace ../shared` works for all services
COPY shared/ /build/shared/

# ---------- api-service dependencies ----------
COPY api-service/go.mod api-service/go.sum ./api-service/
RUN cd api-service && go mod download

# ---------- exif dependencies ----------
COPY exif/go.mod exif/go.sum ./exif/
RUN cd exif && go mod download

# ---------- ocr dependencies ----------
COPY ocr/go.mod ocr/go.sum ./ocr/
RUN cd ocr && go mod download

# ---------- Copy all source code ----------
COPY api-service/ ./api-service/
COPY exif/ ./exif/
COPY ocr/ ./ocr/

# ---------- Build all binaries ----------
RUN cd api-service && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /build/bin/api-service ./cmd/server
RUN cd exif       && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /build/bin/exif ./cmd/server
RUN cd ocr        && CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /build/bin/ocr ./cmd/server

# ======================================================================
# Runtime stages — each pulls only its binary from the common builder
# ======================================================================

# --- api-service runtime ---
FROM alpine:3.24 AS api

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /build/bin/api-service ./api-service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 5170

CMD ["./api-service"]

# --- exif runtime ---
FROM alpine:3.24 AS exif

RUN apk add --no-cache perl exiftool ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/bin/exif ./exif

EXPOSE 5172

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

ENV PORT=${OCR_PORT}
EXPOSE ${OCR_PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost:${OCR_PORT}/ocr/api/health || exit 1

CMD ["./ocr"]
