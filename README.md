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

The checked-in Compose stack starts the API, PostgreSQL and Redis with **development-only** credentials and seed data. For a native Go process, export its environment variables explicitly; it does not read `.env` automatically. See the [quick start](https://fonghehe.github.io/vue-h5-template-business-service/quickstart) for a working database URL and commands.

To run the API from source instead of in Compose (do not start the Compose `business` service as well):

```bash
POSTGRES_PORT=5433 docker compose up -d postgres
export DATABASE_URL='postgres://vue_h5:vue_h5_local@127.0.0.1:5433/vue_h5_business?sslmode=disable'
go run ./cmd/server
```

The bare Go command otherwise uses `localhost:5432`, which may belong to another application. Never migrate or delete that database just to start this service.

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
