# Extend the service

This repository is the **backend**. Adding an H5 page, router menu, Vue component, composable, client store, theme or locale happens in the separate frontend repository, not here. Here the comparable task is adding a Go HTTP endpoint or extending a business transaction.

## Trace an existing endpoint first

`GET /api/cart` is a compact model for a new authenticated read:

1. `internal/httpapi/router.go`: `cart.GET("", s.getCart)` is inside the group using `middleware.Authenticate`.
2. `internal/httpapi/handlers_commerce.go`: `getCart` obtains `middleware.CurrentUserID(c)`, calls `s.services.Cart.List`, then writes `response.OK` or `response.Fail`.
3. `internal/service/commerce_service.go`: `CartService.List` delegates to the repository. Mutating operations in that file enforce quantity, sale status or order state before data access.
4. `internal/repository/commerce.go`: `ListCart` queries with `Where("user_id = ?", userID)` and preloads SKU/Product. Ownership is applied in SQL, not only by hiding fields in the handler.
5. `internal/httpapi/router_test.go` and `internal/service/commerce_test.go` exercise transport and rules separately.

This is the real handler, shortened only by omitting surrounding declarations:

```go
func (s *Server) getCart(c *gin.Context) {
    items, err := s.services.Cart.List(c.Request.Context(), middleware.CurrentUserID(c))
    if err != nil {
        response.Fail(c, err)
        return
    }
    response.OK(c, gin.H{"items": items})
}
```

For a new read: add its route in `router.go`; add a small request/response handler under `internal/httpapi`; put durable rules in `internal/service`; put SQL in `internal/repository`; add router and service tests. Mount authenticated and admin-only routes inside the matching existing group. Do not accept `userId`, price or order status from a client when the authenticated identity or server data is authoritative. Never build SQL by concatenating request text.

## Add a field or persistence entity

1. Define persistence fields and JSON names in `internal/model`. Existing public JSON names are compatibility-sensitive.
2. Add a **new** ordered migration entry in `internal/database/migrate.go`; do not edit a migration that has shipped. Use `internal/database/seed.go` only for deterministic development data. A production migration must not depend on `SEED=true`.
3. Add GORM indexes/constraints for invariants and query patterns. For commerce money use `int64` cents, never float; preserve historical order snapshots.
4. Add repository operations and keep multi-write workflows within `CommerceRepository.Transaction(ctx, fn)`. Reuse conditional inventory updates and `TransitionOrder`; do not assign order status in handlers.
5. Update the API reference in all three locales and the frontend OpenAPI contract when a public endpoint or field changes.

`internal/database/migrate_test.go` demonstrates an existing-product data backfill with `SEED=false`. `internal/service/order_postgres_test.go` checks actual PostgreSQL concurrent orders; SQLite tests alone are not sufficient for row-lock claims.

## Error and response contract

All handlers return `{ code, message, data, error, requestId }` through `internal/response`; `code=0` is success. Construct safe failures with `internal/apierr` (`Validation`, `NotFound`, `Conflict`, etc.). Internal SQL/provider causes are wrapped and logged, not exposed. `internal/httpapi/helpers.go` caps pagination; use the same helper for new list endpoints. The `X-Request-ID` header is echoed and available in service context for business logs.

Authentication is HS256 JWT in `internal/auth`. `middleware.Authenticate` reads the bearer token and attaches user identity; `RequireRole("admin")` gates operator routes. An order or cart lookup must still scope its database query to the authenticated user. Logout clears the refresh cookie but cannot instantly revoke an already-issued stateless access token.

## Tests and checks

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
docker build -t vue-h5-template-business-service:local .
```

The test suites use temporary SQLite files for isolated service/HTTP rules. PostgreSQL-only concurrency tests skip when `TEST_DATABASE_URL` is unset; CI starts PostgreSQL 17 and passes that URL while running `go test -race -coverprofile=coverage.out ./...`. Locally, use a disposable PostgreSQL database or schema for this variable, **never a production database**. `internal/repository/inventory_postgres_test.go` and `internal/service/order_postgres_test.go` create and drop their own test schemas.

The Go project has no TypeScript compilation step. TypeScript type checking is only for the VitePress config:

```bash
cd docs
pnpm install --frozen-lockfile
pnpm docs:typecheck
pnpm docs:build
pnpm docs:dev
```

Docs live at `/`, `/zh/`, `/ja/`; update corresponding pages and sidebar links together. VitePress uses its built-in local search and code highlighting. There is no custom Mermaid renderer: use readable Markdown tables/text for architecture changes rather than relying on diagrams that render as code blocks.
