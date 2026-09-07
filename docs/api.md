# API reference

Business JSON routes, including errors and unknown paths, use the same envelope. `/metrics` returns Prometheus text and CORS preflight replies with HTTP 204, not JSON.

## Response envelope

```jsonc
// success
{ "code": 0, "message": "ok", "data": { "…": "…" }, "error": null, "requestId": "01J…" }

// failure
{ "code": 4010, "message": "username or password is incorrect", "data": null, "error": null, "requestId": "01J…" }
```

The frontend branches on `code === 0`; the HTTP status mirrors the error category but the application `code` is
the authoritative signal.

## Error codes

| Code | Meaning | HTTP |
|---|---|---|
| `0` | Success | 200 / 201 |
| `4000` | Bad request | 400 |
| `4001` | Validation failed | 422 |
| `4010` | Unauthorized | 401 |
| `4030` | Forbidden | 403 |
| `4040` | Not found | 404 |
| `4090` | Conflict | 409 |
| `4130` | Payload too large | 413 |
| `4290` | Rate limited | 429 |
| `5000` | Internal error | 500 |
| `5030` | Unavailable | 503 |

## Authentication

Access tokens are JWTs (`HS256`). Refresh tokens are delivered as an `HttpOnly` cookie (`vh5_refresh` by default) scoped to `/api/auth`, so JavaScript cannot read them. Logout clears that cookie; existing stateless access tokens are not revoked server-side.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/auth/login` | — | Exchange credentials for an access token; sets the refresh cookie. |
| `POST` | `/api/auth/refresh` | refresh cookie or body | Rotate an expired access token. |
| `POST` | `/api/auth/logout` | — | Clear the refresh cookie. |

### POST /api/auth/login

**Body** `{ "username": string, "password": string }`

**Response `data`**

```jsonc
{
  "id": 1,
  "username": "user",
  "realName": "测试用户",
  "avatar": "https://…",
  "roles": ["user"],
  "accessToken": "eyJ…",
  "expiresIn": 7200
}
```

## User

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/user/info` | Bearer | The authenticated account. |
| `GET` | `/api/user/favorites` | Bearer | Bookmarked products, newest first. |

## Product

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/product/list` | — | One page of the public catalogue. |
| `GET` | `/api/product/detail?id=1` | — | A single on-sale product. |
| `POST` | `/api/product/favorite` | Bearer | Bookmark / unbookmark a product. |

### GET /api/product/list

Query params: `page` (default `1`), `pageSize` (default `10`, max `50`), `keyword`, `category`, `priceMin`, `priceMax`, and `sort` (`newest`, `sales`, `price_asc`, `price_desc`; unknown values use featured-first). Invalid/nonpositive page values fall back to defaults and oversize page sizes are clamped to 50. Price filters are non-negative integer cents; both limits must match the same active SKU.

**Response `data`** — a page envelope:

```jsonc
{
  "items": [
    { "id": 1, "title": "…", "imgUrl": "…", "price": "388", "vipPrice": "378",
      "shopDesc": "自营", "delivery": "厂商配送", "shopName": "…", "description": "…" }
  ],
  "total": 14,
  "page": 1,
  "pageSize": 10,
  "hasMore": true
}
```

Legacy `price` / `vipPrice` strings remain for H5 compatibility. Authoritative checkout prices are SKU `int64`
minor units and are always reloaded by the server.

`GET /api/product/detail?id=<productId>` returns the on-sale Product with `skus` (`skuCode`, `name`, JSON `attributes`, integer-cent `price`/`originalPrice`, `status`) and each SKU's current `inventory` (`available`, `reserved`, `version`). Inventory is read from PostgreSQL even when catalogue fields come from Redis. Public list rows do not preload SKUs.

## Cart and orders

All routes in this section require a Bearer access token. Orders are always scoped to the authenticated owner.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/cart` | Load the current user's cart. |
| `POST` | `/api/cart/items` | Add or replace an item (`skuId`, `quantity` 1–99). |
| `PUT` | `/api/cart/items/:id` | Change quantity. |
| `DELETE` | `/api/cart/items/:id` | Remove one item. |
| `DELETE` | `/api/cart` | Clear the cart. |
| `GET` | `/api/orders` | List the current user's orders. |
| `POST` | `/api/orders` | Create an order; requires `Idempotency-Key`. Optional body: `couponCode`. |
| `GET` | `/api/orders/:id` | Fetch one owned order. Foreign orders return 404. |
| `POST` | `/api/orders/:id/cancel` | Cancel a pending order and release reservations. |
| `POST` | `/api/orders/:id/payment` | Create or return a mock payment for an unexpired pending order; no real charge. |

Cart items do not accept or persist a client price. `POST /api/orders` reads active SKUs and prices, reserves
inventory, creates immutable item snapshots, reserves coupon usage and clears the cart in one database transaction.

`POST /api/cart/items` body: `{"skuId":1,"quantity":2}`; it sets that SKU's quantity, rather than adding two to an existing quantity. `PUT /api/cart/items/:id` body: `{"quantity":3}`. Both require quantity 1–99 and a saleable SKU/Product. The cart contains at most 100 distinct items at checkout.

`POST /api/orders` body is `{}` or `{"couponCode":"WELCOME10"}`. The `Idempotency-Key` header is required (max 128 characters). A fresh order returns HTTP 201; a replay of the same key and request returns HTTP 200 with the original order; key reuse with a different coupon code returns 409. `GET /api/orders` uses `page`/`pageSize` (max 50).

## Mock payment callback

`POST /api/payments/mock/webhook` accepts `eventId`, `orderNo`, `reference`, integer-cent `amount`, and `status`
(`SUCCESS` or `FAILURE`). `X-Mock-Signature` must contain the provider HMAC. The database unique key on
`(provider, eventId)` makes callback processing idempotent.

This is a development mock, not a real payment integration. The callback must be signed using the canonical fields described by `MockPaymentProvider.Sign` in `internal/service/payment.go`; `internal/service/payment_test.go` shows the real signing call. An identical event replay succeeds without a second transition; the same event ID with different content returns 409. A `FAILURE` event leaves the order pending for retry or timeout.

## Admin

Operator routes require the `admin` role. They are the only routes that can mutate products.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/admin/products` | Page of the catalogue including hidden items (`status`, `featured` filters). |
| `POST` | `/api/admin/products` | Create a product. |
| `GET` | `/api/admin/products/:id` | Fetch a product regardless of status. |
| `PATCH` | `/api/admin/products/:id` | Partially update a product. |
| `DELETE` | `/api/admin/products/:id` | Soft-remove a product. |
| `POST` | `/api/admin/products/:id/skus` | Create a SKU and initial inventory atomically. |
| `PUT` | `/api/admin/skus/:id` | Replace validated SKU fields. |
| `PUT` | `/api/admin/orders/:id/status` | Advance `PAID → PROCESSING → COMPLETED`. |

### Product payload

`name`, `categoryId`, `brand`, `cover`, `title`, `imgUrl`, `price`, `vipPrice`, `shopDesc`, `delivery`, `shopName`, `description`, `stock`, `status` (`draft` | `on_sale` | `sold_out`), `featured`. The legacy `title`, `imgUrl`, string `price`/`vipPrice` and `shopName` remain required for creation. On `PATCH`, omitted fields stay unchanged. `stock` is a legacy compatibility field, not authoritative inventory.

Create SKU body: `skuCode`, `name`, `attributes` (string map), `price`, `originalPrice` (integer cents), `status` (`active` | `inactive`), `available` (initial stock). The SKU PUT validates the same fields; it does not directly set inventory availability. There is no public coupon-management endpoint. See [Commerce flow](/commerce) for transaction semantics.

## Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness; never touches the database. |
| `GET` | `/ready` | Readiness; fails while the database is unreachable. |
| `GET` | `/metrics` | Prometheus HTTP and commerce metrics. |
| `GET` | `/api/health` | Compatibility liveness path. |
