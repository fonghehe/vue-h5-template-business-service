# Deployment

## Build the image

The image is multi-stage and runs as a **non-root** user, with the build version injected at compile time.

```bash
docker build --build-arg VERSION=$(git describe --tags --always --dirty) \
  -t vue-h5-template-business-service:latest .
```

Or via make:

```bash
make docker
```

## Run with docker compose

```bash
docker compose up -d --build
```

The checked-in Compose file is a **development** stack: API, PostgreSQL 17 (persistent volume), and Redis 8 (cache only, no Redis persistence). It hard-codes development credentials and `APP_ENV=development`; copying `.env.example` does not make it production-ready. The API listens on `http://localhost:8002`. `POSTGRES_PORT=5433 docker compose up -d --build` changes only the database's host port.

## Production checklist

- **Replace every secret** — `JWT_SECRET`, `MOCK_PAYMENT_WEBHOOK_SECRET` (each at least 32 characters), and PostgreSQL credentials. Never ship the Compose defaults.
- **Set `APP_ENV=production`** — this switches on the fail-fast validation below.
- **Terminate TLS upstream** of the app (nginx, a load balancer, or a managed gateway) and set
  `REFRESH_COOKIE_SECURE=true`.
- **Pin `CORS_ORIGINS`** to the real frontend domain(s). Production startup rejects `*` and loopback origins.
- **Use JSON logs** (`LOG_FORMAT=json`) and ship them to your collector.
- **Disable development data** (`SEED=false`); startup rejects `SEED=true` in production. Provision real users/coupons through an approved operational path; there is no public registration or coupon-admin endpoint.
- **Coordinate migrations**: `AUTO_MIGRATE=true` runs recorded migrations on startup; designate one migration owner or run them ahead of replicas. Existing Product rows get a default SKU/inventory via a data migration even with seeding off.
- **Use PostgreSQL as source of truth**. Redis is optional catalogue caching. The in-process IP limiter is per replica; put shared/edge limiting in front of multiple replicas.
- **Constrain `/metrics` at the edge** if needed; the route has no JWT middleware. It exposes HTTP request totals/duration, orders created/paid/expired and inventory reservation failures.
- **Treat payments as a mock**. HMAC callbacks do not settle funds; a real provider needs its own signature verification, event semantics and reconciliation.
- **Keep `JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` identical** to the AI service so tokens minted here validate
  there.
- **Point liveness/readiness** at `/health` and `/ready` respectively.

## Graceful shutdown

On `SIGINT`/`SIGTERM` the server stops accepting connections, drains in-flight requests up to
`SHUTDOWN_TIMEOUT`, then stops the expiration worker and closes the cache client and database pool. A replacement instance resumes timeout processing; PostgreSQL `SKIP LOCKED` prevents replicas claiming the same order concurrently.

## CI

GitHub Actions runs on every PR and push to `main`: format check, `go vet`, `go test -race` with coverage,
`golangci-lint`, and a Docker image build against a PostgreSQL service container.

The separate `deploy-docs.yml` workflow uses pnpm, type-checks VitePress config, builds all three locales and uploads `docs/.vitepress/dist` to GitHub Pages. A successful build does not by itself prove Pages is enabled or publicly reachable.
