# Contributing

Thanks for considering contributing to `vue-h5-template-business-service`.

## Development environment

You need Go 1.25+ and (for local integration) PostgreSQL 17. Everything else is fetched by `go mod`.

```bash
cp .env.example .env
make check
```

`make check` runs formatting, `go vet` and the full test suite. Tests that touch the database use an in-memory
SQLite database and never require a live PostgreSQL instance, so `go test ./...` is self-contained. The CI job
additionally runs the race detector and builds the Docker image against a real PostgreSQL service.

## Workflow

1. Open an issue describing the bug or proposal before starting anything non-trivial.
2. Branch from `main` and keep commits focused and descriptive.
3. Add or update tests for every behavioural change. Run `make test-race` before pushing.
4. Keep the public API and the response envelope stable; when you must change them, update the OpenAPI contract in
   the frontend repository and the VitePress docs in `docs/` in the same change.
5. Run `make fmt` so the CI format gate stays green.

## Code conventions

- Follow the package layout: `config → model → repository → service → httpapi`. Business rules belong in
  `internal/service`, never in HTTP handlers.
- Public JSON uses the shared envelope `{ code, message, data, error, requestId }`. Errors never expose stack
  traces or database internals.
- Store money as integer minor units. Never parse or format currency with floats.
- PostgreSQL is the runtime database; SQLite is reserved for isolated tests.
- Prefer `internal/` packages and exported identifiers with a clear reason to exist.

## Before you open a pull request

- [ ] `make check` passes locally.
- [ ] New behaviour is covered by tests.
- [ ] Docs in `docs/` are updated if the API changed.
- [ ] No secrets or real user data are committed.

## Code of Conduct

This project follows the [Contributor Covenant](./CODE_OF_CONDUCT.md). By participating, you agree to uphold it.
