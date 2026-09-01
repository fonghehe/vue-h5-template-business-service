# Contributing

Thanks for your interest in contributing. This document covers the workflow; behaviour expectations live in
[CODE_OF_CONDUCT.md](https://github.com/fonghehe/vue-h5-template-business-service/blob/main/CODE_OF_CONDUCT.md).

## Setup

```bash
git clone https://github.com/fonghehe/vue-h5-template-business-service.git
cd vue-h5-template-business-service
cp .env.example .env
```

You need Go 1.25+. Tests that touch PostgreSQL run against the CI service container; most unit tests use an
in-memory SQLite database and need no external services.

## Development commands

```bash
make check     # gofmt + go vet + go test
make test-race # go test -race -coverprofile=coverage.out ./...
make lint      # golangci-lint run ./...
make build     # compile the binary into bin/
```

## Code style

- Formatting is enforced by `gofmt`; the CI fails on unformatted code.
- Static analysis runs `go vet` and `golangci-lint` (config in `.golangci.yml`).
- The layered architecture is deliberate — business rules belong in `internal/service`, never in handlers.

## Adding an error code

Application error codes are a **public contract**: once released, a numeric code must not be reused for a
different meaning. Add a new constant in `internal/apierr/errors.go` instead of repurposing an old one.

## Pull request checklist

1. Add or update tests for the change.
2. Run `make check` locally and keep it green.
3. Keep the response envelope contract intact — renaming a JSON field is a breaking change.
4. Update the API reference and configuration docs when the surface changes.

## Releasing

Tag a release and the Docker build will pick up the version via `git describe`.
