# Commerce flow

This page follows the implemented `Product → SKU → Inventory → Cart → Coupon → Order → Payment → Cancel/Timeout` path. It is a single PostgreSQL-backed business service, **not** a real payment gateway or a full marketplace.

## Catalogue, SKU and price

`internal/model/model.go` keeps legacy Product JSON fields (`title`, `imgUrl`, string `price`/`vipPrice`) for the existing H5 client. `internal/model/commerce.go` adds sellable `ProductSKU` rows: `skuCode`, JSON attributes, `price`/`originalPrice` as integer cents, status, and one Inventory row. An admin creates SKUs through `POST /api/admin/products/:id/skus`; the SKU and initial inventory commit together. The public product detail includes SKUs and their current inventory.

`GET /api/product/list` supports keyword, category, active-SKU cent-price range, sort and capped pagination. A price range must match **one** active SKU. Redis, if configured, caches catalogue fields and SKUs; `ProductService.Detail` attaches inventory fresh from PostgreSQL on every read. Product/SKU writes invalidate the cache. `Product.Stock` and legacy string prices are never checkout authority.

## Cart and coupon

The authenticated cart uses `GET /api/cart`, `POST /api/cart/items`, `PUT /api/cart/items/:id`, `DELETE /api/cart/items/:id`, and `DELETE /api/cart`. Add accepts `{"skuId": 1, "quantity": 2}` and replaces that SKU's cart quantity; update accepts `{"quantity": 3}`. Quantity is 1–99, and both Product and SKU must be saleable. A cart row intentionally has **no price**.

The small coupon model supports `FIXED_DISCOUNT`, `PERCENTAGE` (basis points: `1000` = 10%) and `THRESHOLD_DISCOUNT`. Checkout checks time window, minimum amount, global/per-user limits and optional user ownership. `CouponUsage` is `RESERVED` while payment is pending, `CONSUMED` after payment, or `RELEASED` after cancellation/expiration. Demo codes `WELCOME10` and `SAVE20` are seeded only with `SEED=true`; there is no public coupon-creation API.

## Create an order

After login, obtain an access token, read `/api/product/detail?id=1` for a real `skus[].id`, then use that ID:

```bash
curl -X POST http://localhost:8002/api/cart/items \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"skuId":1,"quantity":1}'

curl -X POST http://localhost:8002/api/orders \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Idempotency-Key: checkout-001' \
  -H 'Content-Type: application/json' \
  -d '{"couponCode":"WELCOME10"}'
```

Replace `skuId` with the SKU returned by the detail endpoint. `couponCode` is optional (send `{}` if unused). The header is required, at most 128 characters; use a new key for a new checkout. The response is `201` for a new order and `200` for a replay of the same key/request. Reusing the key with a different coupon code returns `409`.

`OrderService.Create` (`internal/service/commerce_service.go`) runs this PostgreSQL transaction:

```text
BEGIN
  read current cart; reject empty or >100 items
  lock/reload each SKU in stable SKU-ID order; calculate integer-cent total
  lock/validate coupon and usage limits
  reserve every inventory row with conditional UPDATE
  insert order, item snapshots and initial status history
  reserve coupon usage; clear cart
COMMIT (any error rolls back all writes)
```

Snapshots contain `productName`, `skuName`, `unitPrice`, `quantity`; later catalogue edits do not rewrite history. `payableAmount = originalAmount - discountAmount` is computed on the server. The database unique index `(user_id, idempotency_key)` is the final duplicate-order barrier; Redis is not involved in correctness. A retry after a network timeout returns the committed order without another inventory or coupon reservation.

## Stock invariant and state machine

`CommerceRepository.ReserveInventory` performs one conditional SQL update (`available >= quantity`) and checks `RowsAffected`. PostgreSQL evaluates the predicate under its row update lock; DB checks also require `available >= 0` and `reserved >= 0`. Payment success decrements `reserved` (sold stock); cancellation/expiration moves `reserved` back into `available`. No handler directly changes order status.

| From | Allowed next state | Trigger |
|---|---|---|
| `PENDING_PAYMENT` | `PAID` | verified successful callback |
| `PENDING_PAYMENT` | `CANCELLED` | owner calls `/api/orders/:id/cancel` |
| `PENDING_PAYMENT` | `EXPIRED` | background worker after `expiresAt` |
| `PAID` | `PROCESSING` | admin status endpoint |
| `PROCESSING` | `COMPLETED` | admin status endpoint |

`TransitionOrder` in `internal/service/order_state.go` rejects every other transition with `409` and returns a history row that the caller persists in the **same transaction**. `GET /api/orders/:id` is owner-scoped; a foreign order returns `404`.

## Mock payment and duplicate callbacks

`POST /api/orders/:id/payment` creates or returns one mock payment for an owned, unexpired pending order. This does **not** charge a card or mark the order paid. `PaymentProvider` and `MockPaymentProvider` live in `internal/service/payment.go`. The mock webhook is `POST /api/payments/mock/webhook`; it needs `X-Mock-Signature`, an HMAC-SHA256 over the canonical callback fields using `MOCK_PAYMENT_WEBHOOK_SECRET`. It validates order number, payment reference, amount and pending status before a success transition.

The test `TestPaymentWebhookIsIdempotent` in `internal/service/payment_test.go` shows the real signing API (`provider.Sign(callback)`) and sends the same callback ten times. The durable `(provider, event_id)` unique constraint plus payload hash means an identical replay is harmless; reusing an event ID with different content returns `409`. A failure event marks the payment attempt failed but leaves the order pending for retry, cancellation or expiration. This is a **mock** callback endpoint; connect a real provider only after implementing its signature, event semantics and reconciliation rules.

## Expiration and multi-instance behaviour

`cmd/server/main.go` starts the order-expiration goroutine. Every `ORDER_EXPIRATION_INTERVAL`, it selects expired pending orders in batches. PostgreSQL `FOR UPDATE SKIP LOCKED` makes multiple instances claim different rows. For each claimed order, one transaction changes status to `EXPIRED`, releases all reserved stock and coupon usage, and appends history. A concurrent cancellation or successful payment cannot release the same reservation twice. The worker stops during graceful shutdown.

## Test the guarantees

```bash
go test ./internal/service ./internal/repository
go test -race ./...
```

The SQLite tests cover rule-level rollback, coupons, idempotency, webhook replay and state transitions. The PostgreSQL concurrency tests require `TEST_DATABASE_URL`; CI supplies it. `TestPostgresConcurrentOrdersNeverOversell` uses 100 users, 100 carts and stock 10, then checks that exactly 10 orders commit and stock never goes negative. See [Development](/development) for the test setup and [Deployment](/deployment) for operational limits.
