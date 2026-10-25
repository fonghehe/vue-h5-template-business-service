# Configuration

At startup, the server loads an optional `.env` file from the current working directory, then reads configuration from the process environment. Existing process variables take precedence, including explicitly empty values. A missing file is allowed; an unreadable or malformed file aborts startup. Configuration is
**fail-fast**: an invalid or unsafe value aborts startup instead of silently falling back to a default.

There is no `.env.development` or `.env.production` loader. Copy `.env.example` to `.env` in the project root and configure `DATABASE_URL` there. `.env` is ignored by Git; do not commit secrets. Docker Compose reads `.env` only for Compose interpolation (here mainly `POSTGRES_PORT`); `docker-compose.yml` sets the API's environment explicitly. Native startup needs no shell exports when settings are present in `.env`. `TEST_DATABASE_URL` is read only by PostgreSQL integration tests, not `config.Load()`.

`.env.example` contains the connection settings and development secrets needed for a native run. Omitted settings use the defaults below.

## Full reference

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | One of `local`, `development`, `test`, `staging`, `production`. |
| `PORT` | `8002` | HTTP listen port. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `LOG_FORMAT` | `text` outside staging/production, `json` in staging/production | `json` is required in staging/production. |
| `DATABASE_DRIVER` | `postgres` | PostgreSQL for runtime; SQLite only with `APP_ENV=test`. |
| `DATABASE_URL` | `postgres://…` | Driver-specific DSN. |
| `DB_MAX_OPEN_CONNS` | `25` | Max open database connections. |
| `DB_MAX_IDLE_CONNS` | `5` | Max idle connections kept in the pool. |
| `DB_CONN_MAX_LIFETIME` | `30m` | How long a connection may be reused. |
| `AUTO_MIGRATE` | staging/production `false`; otherwise `true` | Run schema migrations at startup. |
| `SEED` | `true` outside staging/production, `false` in staging/production | Load demo data on an empty database; production rejects `true`. |
| `JWT_SECRET` | dev placeholder | **Must** match the AI service. Min 32 chars. |
| `JWT_ISSUER` | `vue-h5-template` | `iss` claim; must match the AI service. |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` claim; must match the AI service. |
| `ACCESS_TOKEN_TTL` | `2h` | Access token lifetime. |
| `REFRESH_TOKEN_TTL` | `336h` | Refresh token lifetime (must exceed `ACCESS_TOKEN_TTL`). |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | HttpOnly cookie carrying the refresh token; its name must pass Go net/http validation or startup fails. |
| `REFRESH_COOKIE_SECURE` | `false` outside staging/production, `true` in staging/production | Mark the refresh cookie `Secure`. |
| `CORS_ORIGINS` | `http://localhost:5173,…` | Comma-separated browser origin allow-list. |
| `RATE_LIMIT_ENABLED` | `true` | Turn the per-IP token bucket on. |
| `RATE_LIMIT_RPS` | `20` | Sustained requests-per-second allowance per IP. |
| `RATE_LIMIT_BURST` | `40` | Bucket capacity per IP. |
| `ORDER_PAYMENT_TTL` | `15m` | Pending-payment reservation lifetime. |
| `ORDER_EXPIRATION_INTERVAL` | `30s` | Expiration worker cadence. |
| `ORDER_EXPIRATION_BATCH_SIZE` | `100` | Maximum orders attempted per worker pass (max 1000); each order has its own transaction. |
| `MOCK_PAYMENT_WEBHOOK_SECRET` | dev placeholder | HMAC secret for mock callbacks; replace in staging/production. |
| `REDIS_URL` | empty | Optional Redis URL for product-detail cache-aside. |
| `PRODUCT_CACHE_TTL` | `5m` | Product-detail cache lifetime. |
| `SHUTDOWN_TIMEOUT` | `15s` | Bound on graceful shutdown. |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP server timeouts. |

## Production validation

When `APP_ENV=production` or `APP_ENV=staging`, startup additionally refuses to run unless all of the following hold:

- `JWT_SECRET` is not the default placeholder and is at least 32 characters.
- `MOCK_PAYMENT_WEBHOOK_SECRET` is not the default placeholder and is at least 32 characters.
- `REFRESH_COOKIE_SECURE` is `true`.
- `CORS_ORIGINS` lists explicit origins — never `*`, and never a loopback origin (`localhost`, `127.0.0.1`, …).
- `LOG_FORMAT` is `json`.
- `SEED` is `false`; enabling demo credentials or coupons in staging/production is rejected.

This is deliberate: a misconfigured service that boots is far more dangerous than one that crashes on launch.

## Parsing and resource limits

Defaults apply only to absent variables. Explicitly empty or malformed integers, booleans and durations fail validation; an empty `DATABASE_URL` is rejected. Ports must be 1–65535 (`0` is allowed only in tests), database idle connections must not exceed open connections, and HTTP/shutdown timeouts must be positive. Rate limits reject NaN and infinity. Log levels and formats must use the supported values above.

## SQLite for isolated tests

SQLite is accepted only with `APP_ENV=test`; development and production use PostgreSQL. Package-local SQLite tests require CGO. Tests that prove row-lock and concurrent checkout behaviour require `TEST_DATABASE_URL` pointing to a dedicated PostgreSQL database; each test creates and removes its own isolated schema.

Deployment settings and migration/refresh-session rollout are described in [Production behavior](/production). `local` is supported for local development; `staging` applies production safeguards. `AUTO_MIGRATE` defaults to false in staging/production and true elsewhere. `DB_CONN_MAX_IDLE_TIME` defaults to 5m; `REQUEST_TIMEOUT` defaults to 10s and must be less than `WRITE_TIMEOUT`.
