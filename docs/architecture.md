# Architecture

This is one Go process with Gin routing, GORM repositories and PostgreSQL. There is no frontend `src/`, page router, component library, client-side store, BFF, WebSocket or SSE implementation in this repository. The optional Redis connection caches catalogue data only; the AI streaming service is a separate repository.

```
cmd/server          entrypoint: wiring, lifecycle, graceful shutdown
internal/config     fail-fast environment configuration
internal/model      persistence entities (some retain legacy JSON field tags)
internal/database   connection, versioned migration, idempotent seed
internal/repository data access (GORM)
internal/service    business rules — the only layer that makes decisions
internal/httpapi    Gin router, handlers, middleware
internal/response   the shared JSON envelope
internal/apierr     stable application error codes
internal/auth       JWT issue / verify
internal/logging    structured JSON / text logging
internal/cache      optional Redis product-detail cache-aside
internal/metrics    private Prometheus registry
docs/               standalone VitePress site (pnpm)
```

The request path is `H5 client → Gin middleware/handler → service → repository → PostgreSQL`. The product-detail service may read catalogue fields from Redis, but attaches inventory freshly from PostgreSQL. The order-expiration goroutine calls the same order service without HTTP. `cmd/server/main.go` starts both and handles shutdown.

## Request lifecycle

1. **Middleware chain** assigns a request id, sets security headers, recovers panics, logs access, records Prometheus metrics, applies CORS and enforces the body limit. The `/api` group additionally uses an in-process per-IP limiter when enabled.
2. **Handler** binds input only, then calls a service method.
3. **Service** enforces business rules and returns an `*apierr.Error` on failure.
4. **Handler** writes the envelope via `response.OK` / `response.Fail`.
5. **Unknown routes** return the same JSON envelope. Authenticated groups verify JWTs; admin routes additionally require the `admin` role.

## Error model

Every failure is an `apierr.Error` carrying three independent pieces of information:

- an **HTTP status** (for clients, load balancers, observability);
- a stable **application code** (`4xxx` / `5xxx`), safe to branch on in client code;
- a **human-readable message**, safe to display to end users.

Unknown errors are normalised to an opaque `5000` — the original cause is logged but never leaked to clients.

## Authentication flow

- **Login** verifies credentials (constant-time bcrypt, dummy-hash on the miss path to flatten timing), then
  mints an access token and a refresh token. The refresh token is written to an `HttpOnly` cookie; the access
  token is returned in the body.
- **Access tokens** are short-lived (`2h`) and sent as `Authorization: Bearer …`.
- **Refresh** issues a new token pair using the cookie (or a body-supplied refresh token for non-browser clients). There is no server-side refresh-token/session store or immediate access-token revocation: logout clears the cookie and clients must discard the access token.
- If another service accepts this service's JWTs, configure a matching `JWT_SECRET`, `JWT_ISSUER` and
  `JWT_AUDIENCE` there. This repository does not implement or verify the separate AI service.

## Data

- PostgreSQL is the runtime source of truth; SQLite is only for isolated tests and demos. SQLite cannot prove PostgreSQL locking behaviour.
- `internal/database/migrate.go` applies ordered, recorded migrations when `AUTO_MIGRATE=true`; a separate data migration backfills default SKUs for legacy products. `internal/database/seed.go` adds demo users/catalogue/coupons only when `SEED=true` and does not overwrite populated tables.
- SKU, order and payment amounts use `int64` minor units. Legacy Product price strings remain compatibility fields.

## Commerce consistency

- Inventory reservation is a conditional atomic update (`available >= quantity`), never a read/check/write sequence.
- Order creation, item snapshots, inventory and coupon reservations, and cart clearing commit or roll back together.
- Order idempotency is enforced by `(user_id, idempotency_key)` plus a request fingerprint.
- Webhook idempotency is enforced by `(provider, event_id)` with `ON CONFLICT DO NOTHING`.
- Cancellation and timeout release inventory and coupon reservations in the order-state transaction.
- Replicated expiration workers use `FOR UPDATE SKIP LOCKED`; version-guarded updates provide an additional race check.
- Redis contains only product catalogue fields and SKUs. Product detail attaches fresh inventory from PostgreSQL on every request; orders and inventory are never cached.

For the concrete checkout, payment and timeout sequence, see [Commerce flow](/commerce). To add a handler/service/repository slice, see [Extend the service](/development).
