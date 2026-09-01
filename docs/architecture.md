# Architecture

The service follows a strict layered architecture. Dependencies point one way — from the transport down to the
database — so rules that outlive HTTP never leak into handlers.

```
cmd/server          entrypoint: wiring, lifecycle, graceful shutdown
internal/config     fail-fast environment configuration
internal/model      persisted entities (public JSON contract)
internal/database   connection, versioned migration, idempotent seed
internal/repository data access (GORM)
internal/service    business rules — the only layer that makes decisions
internal/httpapi    Gin router, handlers, middleware
internal/response   the shared JSON envelope
internal/apierr     stable application error codes
internal/auth       JWT issue / verify
internal/logging    structured JSON / text logging
```

## Request lifecycle

1. **Middleware chain** assigns a request id, sets security headers, recovers panics, logs access, applies CORS
   and enforces the body limit.
2. **Handler** binds input only, then calls a service method.
3. **Service** enforces business rules and returns an `*apierr.Error` on failure.
4. **Handler** writes the envelope via `response.OK` / `response.Fail`.
5. **Unknown routes** return the same JSON envelope — the API never emits HTML.

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
- **Refresh** rotates the access token using the cookie (or a body-supplied refresh token for non-browser clients).
- The **AI service** validates the same access tokens — `JWT_SECRET`, `JWT_ISSUER` and `JWT_AUDIENCE` must be
  identical across both services.

## Data

- PostgreSQL is the production database; SQLite is used for isolated tests and demos.
- Schema is managed by a versioned migration, run automatically when `AUTO_MIGRATE` is set.
- The seed is idempotent — it only fills empty tables, so it never overwrites real data.
- Monetary values are stored as strings to avoid floating-point drift.
