# AGENTS.md

This repository is the conventional **business API** for vue-h5-template: authentication, user profile and
favourites, and the product catalogue. AI model and streaming code belong in the sibling
`vue-h5-template-ai-service` repository.

## Boundaries

- `internal/httpapi`: Gin routing, validation, auth middleware, and response mapping.
- `internal/service`: business rules that outlive HTTP.
- `internal/repository`: data access on top of GORM.
- `internal/database`: versioned migrations and deterministic development seeds.
- `internal/model`: persistence models only.
- `internal/config`: environment parsing and fail-fast validation.
- Public JSON uses `{ code, message, data, error, requestId }`; `code === 0` means success. Errors never expose
  internal details.
- PostgreSQL is the runtime database. SQLite is allowed only for isolated unit tests.
- Store money as integer minor units (¥ cents).

## Commands

```bash
gofmt -w .
go vet ./...
go test -race ./...
docker build -t vue-h5-template-business-service:local .
```

When a public API changes, update the tests, the OpenAPI contract in the frontend repository, and the VitePress
docs in `docs/` in the same change.
