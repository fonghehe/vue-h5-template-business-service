# Configuration

All configuration is read from the process environment; the binary does not load `.env` automatically. Configuration is
**fail-fast**: an invalid or unsafe value aborts startup instead of silently falling back to a default.

There is no `.env.development` or `.env.production` loader in this Go service. `.env.example` is a template, not runtime configuration. Docker Compose reads `.env` only for Compose interpolation (here mainly `POSTGRES_PORT`); `docker-compose.yml` sets the API's environment explicitly. A native process must receive exported variables. `TEST_DATABASE_URL` is read only by PostgreSQL integration tests, not `config.Load()`.

## Full reference

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | One of `development`, `test`, `production`. |
| `PORT` | `8002` | HTTP listen port. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `LOG_FORMAT` | `text` outside production, `json` in production | `json` is required in production. |
| `DATABASE_DRIVER` | `postgres` | `postgres` or `sqlite` (sqlite is for isolated demos/tests only). |
| `DATABASE_URL` | `postgres://…` | Driver-specific DSN. |
| `DB_MAX_OPEN_CONNS` | `25` | Max open database connections. |
| `DB_MAX_IDLE_CONNS` | `5` | Max idle connections kept in the pool. |
| `DB_CONN_MAX_LIFETIME` | `30m` | How long a connection may be reused. |
| `AUTO_MIGRATE` | `true` | Run schema migrations at startup. |
| `SEED` | `true` outside production, `false` in production | Load demo data on an empty database; production rejects `true`. |
| `JWT_SECRET` | dev placeholder | **Must** match the AI service. Min 32 chars. |
| `JWT_ISSUER` | `vue-h5-template` | `iss` claim; must match the AI service. |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` claim; must match the AI service. |
| `ACCESS_TOKEN_TTL` | `2h` | Access token lifetime. |
| `REFRESH_TOKEN_TTL` | `336h` | Refresh token lifetime (must exceed `ACCESS_TOKEN_TTL`). |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | HttpOnly cookie carrying the refresh token. |
| `REFRESH_COOKIE_SECURE` | `false` outside production, `true` in production | Mark the refresh cookie `Secure`. |
| `CORS_ORIGINS` | `http://localhost:5173,…` | Comma-separated browser origin allow-list. |
| `RATE_LIMIT_ENABLED` | `true` | Turn the per-IP token bucket on. |
| `RATE_LIMIT_RPS` | `20` | Sustained requests-per-second allowance per IP. |
| `RATE_LIMIT_BURST` | `40` | Bucket capacity per IP. |
| `ORDER_PAYMENT_TTL` | `15m` | Pending-payment reservation lifetime. |
| `ORDER_EXPIRATION_INTERVAL` | `30s` | Expiration worker cadence. |
| `ORDER_EXPIRATION_BATCH_SIZE` | `100` | Rows claimed per worker transaction (max 1000). |
| `MOCK_PAYMENT_WEBHOOK_SECRET` | dev placeholder | HMAC secret for mock callbacks; replace in production. |
| `REDIS_URL` | empty | Optional Redis URL for product-detail cache-aside. |
| `PRODUCT_CACHE_TTL` | `5m` | Product-detail cache lifetime. |
| `SHUTDOWN_TIMEOUT` | `15s` | Bound on graceful shutdown. |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP server timeouts. |

## Production validation

When `APP_ENV=production`, startup additionally refuses to run unless all of the following hold:

- `JWT_SECRET` is not the default placeholder and is at least 32 characters.
- `MOCK_PAYMENT_WEBHOOK_SECRET` is not the default placeholder and is at least 32 characters.
- `REFRESH_COOKIE_SECURE` is `true`.
- `CORS_ORIGINS` lists explicit origins — never `*`, and never a loopback origin (`localhost`, `127.0.0.1`, …).
- `LOG_FORMAT` is `json`.
- `SEED` is `false`; enabling demo credentials or coupons in production is rejected.

This is deliberate: a misconfigured service that boots is far more dangerous than one that crashes on launch.

## SQLite for isolated runs

`DATABASE_DRIVER=sqlite` is supported for isolated tests and local demos, not the production runtime. Concurrency guarantees that depend on PostgreSQL row locks require PostgreSQL tests.

```bash
DATABASE_DRIVER=sqlite DATABASE_URL=/tmp/vue-h5-business.db make run
```
