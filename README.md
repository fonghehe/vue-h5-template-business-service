# vue-h5-template-business-service

English | [简体中文](./README.zh-CN.md) | [日本語](./README.ja.md)

Go/Gin commerce API for vue-h5-template. This repository owns authentication, profile, favourites, the product catalogue, SKU inventory, cart, coupons, orders and mock payment state. The H5 UI and streaming AI service live in separate repositories.

The checkout path is `Product → SKU → Inventory → Cart → Coupon → Order → Payment → Cancel/Timeout`. PostgreSQL is the source of truth: order creation, inventory and coupon reservations, snapshots and cart clearing commit together. Database unique constraints make order retries and payment callbacks safe; an expiration worker releases unpaid reservations. Optional Redis caches catalogue data only, never authoritative inventory or orders.

## Stack

Go 1.25, Gin, GORM, PostgreSQL 17, optional Redis, JWT, Prometheus, VitePress documentation. SQLite is used for isolated tests, not production. Payment is a signed **mock provider**, not a real payment integration.

## Quick start

```bash
docker compose up -d --build
curl http://localhost:8002/health
curl http://localhost:8002/ready
curl 'http://localhost:8002/api/product/list?page=1&pageSize=10'
```

The checked-in Compose stack starts the API, PostgreSQL and Redis with **development-only** credentials and seed data. For native development, copy `.env.example` to `.env`; the server loads this project file automatically. Existing process environment variables take precedence. See the [quick start](https://fonghehe.github.io/vue-h5-template-business-service/quickstart) for a working database URL and commands.

To run the API from source instead of in Compose (do not start the Compose `business` service as well):

```bash
cp .env.example .env # once; edit DATABASE_URL for your dedicated PostgreSQL
POSTGRES_PORT=5433 docker compose up -d --wait postgres # if needed
go run ./cmd/server
```

The example `.env` uses the dedicated development database on port 5433. If PostgreSQL is already available locally, set `DATABASE_URL` in `.env` and skip the Compose command. Without `.env` or an explicit `DATABASE_URL`, the Go command falls back to `localhost:5432`, which may belong to another application. Never migrate or delete that database just to start this service.

## Interactive API documentation

Swagger UI is available at [http://localhost:8002/swagger/index.html](http://localhost:8002/swagger/index.html). The current OpenAPI contract is served at `/openapi.yaml`. The page is registered with `github.com/swaggo/gin-swagger`, and `github.com/swaggo/files` supplies its assets. No separate documentation server or CDN is required. The middleware reads the existing OpenAPI 3 contract; it does not require generating a Swagger 2 document with `swag init`. These routes are disabled when `APP_ENV=production`.

Sign in through `POST /api/auth/login` using the development account `user` / `123456`, then copy `data.accessToken` into **Authorize**. Use `admin` / `123456` for administration endpoints. Mock payments never charge real money.

## Check

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
```

The PostgreSQL 100-buyer/10-stock tests require `TEST_DATABASE_URL`; CI provides an isolated test database. Docs are maintained in English, Simplified Chinese and Japanese:

- [Documentation](https://fonghehe.github.io/vue-h5-template-business-service/)
- [中文文档](https://fonghehe.github.io/vue-h5-template-business-service/zh/)
- [日本語ドキュメント](https://fonghehe.github.io/vue-h5-template-business-service/ja/)

Locally: `cd docs && pnpm install --frozen-lockfile && pnpm docs:dev`.

## License

[MIT](./LICENSE)

## Architecture and production operations

Keep `Handler → Service → Repository → PostgreSQL`; explicit HTTP DTOs preserve the client contract. `make migrate` applies append-only migrations and `make migrate-status` shows pending versions without starting HTTP or seeding. Staging/production default to `AUTO_MIGRATE=false`; migrate before starting replicas. The separate `compose.production.yml` runs the image against an externally supplied dedicated database. Read [production behavior and upgrade](docs/production.md) before deploying. Refresh sessions now rotate once and can be revoked on logout; users with old refresh tokens must sign in again.
