# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.5
ARG ALPINE_VERSION=3.22
ARG ATLAS_VERSION=1.3.2

############################
# Atlas migration binary
############################
FROM arigaio/atlas:${ATLAS_VERSION}-community-alpine AS atlas

############################
# Build Sigryx
############################
FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache \
    build-base \
    ca-certificates \
    libsodium-dev \
    pkgconf

ARG GOPROXY=https://proxy.golang.org,direct

ENV CGO_ENABLED=1 \
    GOFLAGS=-trimpath \
    GOPROXY=${GOPROXY} \
    GO111MODULE=on

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/sigryx ./cmd

############################
# Single runtime image
#
# Contains:
#   - Sigryx
#   - Atlas
#   - versioned migrations
#
# By default the entrypoint applies pending Atlas migrations and then starts
# Sigryx. Set POSTGRES_AUTO_MIGRATE=false when migrations are managed by a
# separate deployment step.
############################
FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache \
    ca-certificates \
    libsodium \
    postgresql-client \
    tzdata \
    && addgroup -S -g 10001 sigryx \
    && adduser -S -D -H -u 10001 -G sigryx sigryx

WORKDIR /app

COPY --from=build /out/sigryx /usr/local/bin/sigryx
COPY --from=atlas /atlas /usr/local/bin/atlas
COPY --chown=sigryx:sigryx atlas.hcl /app/atlas.hcl
COPY --chown=sigryx:sigryx migrations /app/migrations
COPY --chown=sigryx:sigryx docker/entrypoint.sh /usr/local/bin/sigryx-entrypoint

RUN chmod 0555 \
    /usr/local/bin/sigryx \
    /usr/local/bin/atlas \
    /usr/local/bin/sigryx-entrypoint

ENV SERVICE_NAME=sigryx \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    POSTGRES_SCHEMA=vault \
    POSTGRES_AUTO_MIGRATE=true \
    POSTGRES_MAX_CONNS=10 \
    POSTGRES_MIN_CONNS=2 \
    POSTGRES_CONNECT_TIMEOUT=5s \
    POSTGRES_MAX_CONN_LIFETIME=30m \
    POSTGRES_MAX_CONN_IDLE_TIME=5m \
    HTTP_ADDR=0.0.0.0:8080 \
    HTTP_READ_TIMEOUT=15s \
    HTTP_WRITE_TIMEOUT=15s \
    HTTP_IDLE_TIMEOUT=60s \
    HTTP_SHUTDOWN_TIMEOUT=15s \
    AUTH_ISSUER=sigryx \
    AUTH_AUDIENCE=sigryx-api \
    AUTH_ACCESS_TTL=10m \
    AUTH_REFRESH_TTL=168h \
    AUDIT_NORMAL_RETENTION_DAYS=30 \
    AUDIT_CRITICAL_RETENTION_DAYS=365 \
    AUDIT_CLEANUP_INTERVAL=6h \
    AUDIT_CLEANUP_BATCH_SIZE=5000 \
    ALERT_WEBHOOK_TIMEOUT=2s \
    MAX_UNSEAL_SIZE=10 \
    HOME=/tmp

EXPOSE 8080

USER sigryx:sigryx

ENTRYPOINT ["/usr/local/bin/sigryx-entrypoint"]
CMD ["sigryx"]
