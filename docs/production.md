# Production behavior and upgrade

The service remains a layered Go monolith. HTTP DTOs are explicit; handlers bind and map requests, services own rules and transaction boundaries, and repositories execute contextual SQL. AI streaming stays in its sibling service. Existing `/api` endpoints and JSON field names remain available; `/api/v1` registers the same business handlers. Both aliases share rate limiters. Cookies are scoped to the auth prefix used for login.

## Upgrade an existing installation

1. Back up the dedicated PostgreSQL database and rehearse on a restored copy.
2. Run `make migrate-status` and `make migrate` with the new binary and reviewed environment.
3. New append-only migrations create `refresh_sessions` and enforce commerce constraints/indexes. Invalid existing rows stop the transaction; investigate them rather than disabling checks. A PostgreSQL transaction advisory lock serializes migration owners within the same database/schema. The complete pending batch rolls back on failure. There is no destructive automatic down migration: restore a verified backup or issue a reviewed forward fix.
4. Start replicas with `AUTO_MIGRATE=false` and `SEED=false`. The process refuses pending migrations before serving HTTP. Production and staging use these defaults. Development can still migrate/seed on startup; seeding is serialized and only fills empty tables.
5. Existing access JWTs remain usable if their accounts are active. Existing refresh JWTs have no stored session and require a fresh login. Coordinate this behavior with the frontend release.

Migration IDs use a sequential number and a descriptive name (for example, `004_extend_product_catalogue`), without dates. Existing date-based ledger entries are still recognized as applied and need no manual rewrite; newly applied migrations record the new IDs. Append future migrations without reusing or changing released steps.

## Authentication and browser boundaries

Refresh JWTs are stored only as SHA-256 hashes in PostgreSQL. Refresh consumes one live session and inserts its replacement in one transaction; concurrent use has one winner. Logout revokes the presented refresh token and clears its cookie. Access tokens remain stateless until expiry and must be discarded by clients. This is current-session logout, not global logout or token-family compromise detection. A stolen replacement token can still be used until revoked/expired.

Every authenticated business request loads current account status and roles; disabling the account or removing `admin` takes effect here on its next request. The AI service must implement its own equivalent policy. The present role model (`user`, `admin`) fits current endpoints; no permission-management UI or dynamic permission tables were added.

Browsers must send refresh cookies with credentials and serialize refresh calls. Only the configured browser origins are accepted. Requests without Origin support non-browser clients; CORS is not authentication. JSON refresh/logout bodies accept an optional `refreshToken`. Malformed or trailing JSON returns 422, including on cookie-backed requests.

## Resource and operational limits

- `REQUEST_TIMEOUT=10s` propagates a cooperative deadline to SQL/cache work and returns HTTP 504 / code 5040 on expiry. It must be shorter than `WRITE_TIMEOUT`. It does not forcibly stop CPU work that ignores context or guarantee cancellation rolls back a transaction already committed; retry checkout with the same idempotency key.
- HTTP request bodies are capped at 1 MiB, including chunked requests and trailing data. Page sizes stay capped at 50. The per-instance limiter holds at most 10,000 client buckets; unknown clients are rejected at capacity until idle entries expire. Auth endpoints share an additional 0.5 requests/second, burst 5 limiter. Use shared edge controls for replicas; this server does not trust arbitrary forwarded IP headers.
- `/metrics` includes HTTP counters/histograms, business counters, database pool statistics and Go runtime collectors; process statistics depend on platform support. Keep this unauthenticated endpoint on a private network or restrict it at the proxy. OpenTelemetry export and a tracing backend are not configured.
- SQL errors/slow-query durations use structured logs with requestId without raw SQL or bound parameters. Request IDs propagate through a transport-neutral context helper. The worker cleans expired sessions in bounded batches, expires orders independently and respects cancellation. On a signal, workers are cancelled; after HTTP drains, the process waits for workers before closing cache/database resources.
- Redis remains an optional catalogue cache. Connection/pool/command waits are bounded, request deadlines are respected, retries are disabled and TTLs have jitter. Inventory remains a fresh PostgreSQL read. Cache invalidation is best-effort; catalogue changes can remain stale until TTL after an outage or concurrent refill. Readiness depends on PostgreSQL because Redis is optional.

## Run the reviewed production image

The default `docker-compose.yml` remains a local development stack. The separate production definition requires external configuration and an existing dedicated database:

```bash
docker build -t vue-h5-template-business-service:local .
# DATABASE_URL, JWT_SECRET, MOCK_PAYMENT_WEBHOOK_SECRET and CORS_ORIGINS
# must be supplied by the deployment environment; never use development secrets.
docker compose -f compose.production.yml run --rm migrate
docker compose -f compose.production.yml up -d business
```

The application binds the host port to localhost, runs as a non-root user with a read-only filesystem and dropped capabilities, and needs an upstream TLS proxy. It does not provision production users, TLS certificates, database backups or monitoring storage. Both `/app/server` and `/app/migrate` are in the image. Health probes use `/health` and `/ready`.

## Remaining commercial boundary

The payment provider is explicitly a simulator. It performs no real settlement. Real payment rollout needs provider-specific signing, idempotent external intent creation outside long database transactions, callback retry policy, reconciliation and refund flows. No MQ, cron platform or unused external-client abstraction is introduced without a business requirement.
