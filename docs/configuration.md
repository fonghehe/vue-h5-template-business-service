# Configuration

All configuration is read from the environment (optionally seeded from a `.env` file). Configuration is
**fail-fast**: an invalid or unsafe value aborts startup instead of silently falling back to a default.

## Full reference

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | One of `development`, `test`, `production`. |
| `PORT` | `8002` | HTTP listen port. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `LOG_FORMAT` | `text` | `text` for local dev, `json` (required in production). |
| `DATABASE_DRIVER` | `postgres` | `postgres` or `sqlite` (sqlite is for isolated demos/tests only). |
| `DATABASE_URL` | `postgres://…` | Driver-specific DSN. |
| `DB_MAX_OPEN_CONNS` | `25` | Max open database connections. |
| `DB_MAX_IDLE_CONNS` | `5` | Max idle connections kept in the pool. |
| `DB_CONN_MAX_LIFETIME` | `30m` | How long a connection may be reused. |
| `AUTO_MIGRATE` | `true` | Run schema migrations at startup. |
| `SEED` | `true` | Load demo data on an empty database. |
| `JWT_SECRET` | dev placeholder | **Must** match the AI service. Min 32 chars. |
| `JWT_ISSUER` | `vue-h5-template` | `iss` claim; must match the AI service. |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` claim; must match the AI service. |
| `ACCESS_TOKEN_TTL` | `2h` | Access token lifetime. |
| `REFRESH_TOKEN_TTL` | `336h` | Refresh token lifetime (must exceed `ACCESS_TOKEN_TTL`). |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | HttpOnly cookie carrying the refresh token. |
| `REFRESH_COOKIE_SECURE` | `false` | Mark the refresh cookie `Secure` (must be `true` in production). |
| `CORS_ORIGINS` | `http://localhost:5173,…` | Comma-separated browser origin allow-list. |
| `RATE_LIMIT_ENABLED` | `true` | Turn the per-IP token bucket on. |
| `RATE_LIMIT_RPS` | `20` | Sustained requests-per-second allowance per IP. |
| `RATE_LIMIT_BURST` | `40` | Bucket capacity per IP. |
| `SHUTDOWN_TIMEOUT` | `15s` | Bound on graceful shutdown. |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP server timeouts. |

## Production validation

When `APP_ENV=production`, startup additionally refuses to run unless all of the following hold:

- `JWT_SECRET` is not the default placeholder and is at least 32 characters.
- `REFRESH_COOKIE_SECURE` is `true`.
- `CORS_ORIGINS` lists explicit origins — never `*`, and never a loopback origin (`localhost`, `127.0.0.1`, …).
- `LOG_FORMAT` is `json`.

This is deliberate: a misconfigured service that boots is far more dangerous than one that crashes on launch.

## SQLite for isolated runs

`DATABASE_DRIVER=sqlite` is supported for demos and isolated tests, where a full PostgreSQL is unnecessary.
It is **not** intended for production — use PostgreSQL there.

```bash
DATABASE_DRIVER=sqlite DATABASE_URL=/tmp/vue-h5-business.db make run
```
