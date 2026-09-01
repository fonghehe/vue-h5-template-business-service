# Quick start

Get the business service running locally in under a minute, either with Docker Compose (recommended) or by
running the Go binary directly against a local PostgreSQL.

## Prerequisites

- **Docker** and **Docker Compose** — for the containerised path; or
- **Go 1.25+** and **PostgreSQL 16+** — for the bare-metal path.

## Option A — Docker Compose

This starts the service together with a PostgreSQL 17 instance, runs the migration and seeds the demo data.

```bash
cp .env.example .env
docker compose up --build
```

The API listens on `http://localhost:8002`. Verify it:

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
```

## Option B — run from source

```bash
cp .env.example .env
# Point DATABASE_URL at your local PostgreSQL, then:
make run
```

`make run` executes `go run ./cmd/server`. Configuration is read from the environment; the values in `.env`
are documented in the [configuration reference](/configuration).

## Try the API

Log in with a seeded account and read the catalogue:

```bash
# 1. Authenticate (returns accessToken; also sets the refresh cookie)
curl -s http://localhost:8002/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"user","password":"123456"}'

# 2. Read your profile with the token from step 1
curl -s http://localhost:8002/api/user/info \
  -H 'Authorization: Bearer <accessToken>'

# 3. Bookmark a product
curl -s http://localhost:8002/api/product/favorite \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"productId":1,"favorite":true}'
```

Interactive, non-streaming endpoints are documented in the [API reference](/api).

## Development loop

```bash
make check     # format + vet + test
make test-race # race detector + coverage
make lint      # golangci-lint
```

## Next steps

- [Configuration reference](/configuration) — every environment variable.
- [Architecture](/architecture) — how the layers fit together.
- [Deployment](/deployment) — shipping to production.
