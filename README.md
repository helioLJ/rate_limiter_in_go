# Rate Limiter in Go (Redis + Gin)

A production-ready distributed rate limiter for APIs using a Redis-backed token bucket.
Includes a Gin middleware and a reverse-proxy sidecar for protecting upstream services.

## Requirements

- Go 1.21+
- Redis 6+
- Docker (optional, for local Redis)

## Quick start

1) Start Redis:

```bash
docker compose up -d redis
```

2) Run the API (will be added in subsequent steps):

```bash
go run ./cmd/api
```

3) Run the proxy (optional):

```bash
go run ./cmd/proxy
```

## Configuration (defaults)

- `RATE_LIMIT`: 100 requests
- `WINDOW`: 1m
- `BURST`: 0
- `REDIS_ADDR`: `localhost:6379`
- `FAIL_MODE`: `open` (allow on Redis errors) or `closed`
- `KEY_HEADER`: `X-API-Key`
- `UPSTREAM_URL`: `http://localhost:8081` (proxy only)

## Project layout (planned)

- `cmd/api`: sample Gin API
- `cmd/proxy`: reverse proxy sidecar
- `internal/ratelimiter`: token bucket + Redis Lua
- `middleware/gin`: Gin middleware adapter

## Dependencies

- `github.com/redis/go-redis/v9` for Redis access
- `github.com/gin-gonic/gin` for the sample API and middleware

## Verification

```bash
gofmt -w .
go vet ./...
go test ./...
```
