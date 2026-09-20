# Quick start

Run the Go business API. This repository does not contain the H5 UI; its VitePress site is a separate Node project in `docs/`.

## Prerequisites

- **Docker with Compose** for the complete local stack; or Go 1.25+ and a running PostgreSQL for native development.
- **Node.js 22+ and pnpm 11** only when building or editing VitePress documentation.

## Option A — Docker Compose

The checked-in Compose file starts the API, PostgreSQL 17 and Redis 8. Development defaults apply migrations and seed demo users, products, SKUs and coupons. Compose sets the service environment explicitly; copying `.env.example` does **not** configure its JWT, database URL or Redis URL.

```bash
docker compose up -d --build
```

The API listens on `http://localhost:8002`. Verify it:

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
docker compose ps
```

## Option B — run from source

```bash
# Copy once, then configure the dedicated database in the project file.
cp .env.example .env
# Optional: provide PostgreSQL through Compose if no local database is available.
POSTGRES_PORT=5433 docker compose up -d --wait postgres
go run ./cmd/server
```

`make run` invokes the same Go command. Do not start the Compose `business` service at the same time: it also binds host port 8002. The server automatically reads `.env` from the working directory; run from the repository root. Existing process variables take precedence. `.env.example` uses port 5433. If PostgreSQL is already installed, set `DATABASE_URL` in `.env` to a dedicated database and skip the Compose command. Redis is optional for a native API run (`REDIS_URL` defaults to empty). If the default Go module proxy times out, run `GOPROXY=https://goproxy.cn go mod download` before starting; keep the repository's `go.sum` verification enabled. See [Configuration](/configuration).

Without `.env` or an explicit `DATABASE_URL`, running `go run ./cmd/server` targets the default `localhost:5432`, which may be a different application's database. Do **not** migrate or delete its tables to make this service start. Use a dedicated database and configure its URL in the project `.env`. Startup now rejects incompatible `users`/`products` schemas before applying migrations.

## Interactive API documentation

Swagger UI is available at `http://localhost:8002/swagger/index.html`. The current OpenAPI contract is served at `/openapi.yaml`. The page is registered with `github.com/swaggo/gin-swagger`, and `github.com/swaggo/files` supplies its assets. No separate documentation server or CDN is required. The middleware reads the existing OpenAPI 3 contract; it does not require generating a Swagger 2 document with `swag init`. These routes are disabled when `APP_ENV=production`.

Sign in through `POST /api/auth/login` using the development account `user` / `123456`, then copy `data.accessToken` into **Authorize**. Use `admin` / `123456` for administration endpoints. Mock payments never charge real money.

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

For a cart-to-order walkthrough, see [Commerce flow](/commerce). Endpoint details are in [API reference](/api).

## Development loop

```bash
make check     # format + vet + test
make test-race # race detector + coverage
make lint      # golangci-lint
make build     # binary in bin/server

cd docs
pnpm install --frozen-lockfile
pnpm docs:dev
pnpm docs:build
```

## Next steps

- [Configuration reference](/configuration) — every environment variable.
- [Architecture](/architecture) — how the layers fit together.
- [Deployment](/deployment) — shipping to production.
- [Extend the service](/development) — add an endpoint, model and tests.
