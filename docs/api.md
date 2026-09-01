# API reference

The business service exposes a JSON HTTP API. Every endpoint — including errors and unknown routes — returns the
same envelope.

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

Access tokens are JWTs (`HS256`). Refresh tokens are delivered as an `HttpOnly` cookie (`vh5_refresh`) scoped to
the auth paths, so JavaScript can never read them.

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

Query params: `page` (default `1`), `pageSize` (default `10`, max `50`), `keyword`, `sort`.

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

Monetary values (`price`, `vipPrice`) are stored and returned as **strings** so no rounding can occur between the
database and the client.

## Admin

Operator routes require the `admin` role. They are the only routes that can mutate products.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/admin/products` | Page of the catalogue including hidden items (`status`, `featured` filters). |
| `POST` | `/api/admin/products` | Create a product. |
| `GET` | `/api/admin/products/:id` | Fetch a product regardless of status. |
| `PATCH` | `/api/admin/products/:id` | Partially update a product. |
| `DELETE` | `/api/admin/products/:id` | Soft-remove a product. |

### Product payload

`title`, `imgUrl`, `price`, `vipPrice`, `shopDesc`, `delivery`, `shopName`, `description`, `stock`, `status`
(`draft` | `on_sale` | `sold_out`), `featured`. On `PATCH`, omitted fields are left unchanged.

## Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness; never touches the database. |
| `GET` | `/ready` | Readiness; fails while the database is unreachable. |
