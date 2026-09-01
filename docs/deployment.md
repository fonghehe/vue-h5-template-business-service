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
cp .env.example .env
docker compose up --build
```

This brings up the API plus PostgreSQL 17 with a persistent volume and a health check. The API listens on
`http://localhost:8002`.

## Production checklist

- **Replace every secret** — `JWT_SECRET` (min 32 chars, random), and the PostgreSQL password. Never ship the
  defaults.
- **Set `APP_ENV=production`** — this switches on the fail-fast validation below.
- **Terminate TLS upstream** of the app (nginx, a load balancer, or a managed gateway) and set
  `REFRESH_COOKIE_SECURE=true`.
- **Pin `CORS_ORIGINS`** to the real frontend domain(s). Production startup rejects `*` and loopback origins.
- **Use JSON logs** (`LOG_FORMAT=json`) and ship them to your collector.
- **Keep `JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` identical** to the AI service so tokens minted here validate
  there.
- **Point liveness/readiness** at `/health` and `/ready` respectively.

## Graceful shutdown

On `SIGINT`/`SIGTERM` the server stops accepting connections, drains in-flight requests up to
`SHUTDOWN_TIMEOUT`, then closes the database pool and exits.

## CI

GitHub Actions runs on every PR and push to `main`: format check, `go vet`, `go test -race` with coverage,
`golangci-lint`, and a Docker image build against a PostgreSQL service container.
