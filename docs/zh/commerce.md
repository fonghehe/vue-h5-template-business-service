# 交易闭环

这页对应代码中真正实现的 `Product → SKU → Inventory → Cart → Coupon → Order → Payment → Cancel/Timeout`。这是一个以 PostgreSQL 为数据真相的单体商业服务，**没有**接入真实支付渠道，也不是完整电商平台。

## 商品、SKU 与金额

`internal/model/model.go` 保留旧 H5 使用的 Product 字段（`title`、`imgUrl`、字符串 `price` / `vipPrice`）。`internal/model/commerce.go` 中的 ProductSKU 才是可售单位：`skuCode`、JSON 属性、以整数分存储的 `price` / `originalPrice`、状态和一行 Inventory。管理员通过 `POST /api/admin/products/:id/skus` 创建 SKU，初始库存与 SKU 在同一事务提交。公开商品详情返回 SKU 和当前库存。

商品列表支持关键词、分类、active SKU 价格区间、排序和限额分页；区间必须由**同一个** active SKU 满足。可选 Redis 只缓存目录字段和 SKU；`ProductService.Detail` 每次从 PostgreSQL 附加最新库存。商品或 SKU 修改会使缓存失效。旧 Product.Stock 和字符串价格不参与结算。

## 购物车与优惠券

登录后使用 `GET /api/cart`、`POST /api/cart/items`、`PUT /api/cart/items/:id`、`DELETE /api/cart/items/:id`、`DELETE /api/cart`。添加参数 `{"skuId":1,"quantity":2}` 会设置该 SKU 的购物车数量；更新参数为 `{"quantity":3}`。数量限 1–99，Product 和 SKU 都必须可售。购物车表**不保存价格**。

优惠类型只有 `FIXED_DISCOUNT`、`PERCENTAGE`（基点，`1000` = 10%）和 `THRESHOLD_DISCOUNT`。结算时校验有效期、门槛、全局/个人次数及可选的用户归属。待支付时 CouponUsage 为 `RESERVED`，支付成功转 `CONSUMED`，取消或超时转 `RELEASED`。`WELCOME10`、`SAVE20` 仅在 `SEED=true` 时作为演示券写入；目前没有公开的优惠券创建 API。

## 创建订单

登录获取 access token，再访问 `/api/product/detail?id=1` 找到真实 `skus[].id`：

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

把示例 `skuId` 换成详情接口返回值；不使用优惠券时传 `{}`。`Idempotency-Key` 必填、最长 128 字符，每次新的结算要用新 Key。首次成功返回 `201`；同 Key、同请求重试返回原订单（`200`）；同 Key 换优惠券返回 `409`。

`internal/service/commerce_service.go` 的 `OrderService.Create` 在一个 PostgreSQL 事务里执行：

```text
BEGIN
  读取购物车，拒绝空车或超过 100 项
  按 SKU ID 排序并锁定/重读最新 SKU，计算整数分总额
  锁定并校验优惠券和使用限制
  逐项条件 UPDATE 预占库存
  写订单、OrderItem 快照、初始状态历史
  预占优惠券，清空购物车
COMMIT；任一步失败均回滚
```

OrderItem 保存 `productName`、`skuName`、`unitPrice`、`quantity`，以后改商品不会改历史订单。`payableAmount = originalAmount - discountAmount` 由服务端计算。`(user_id, idempotency_key)` 数据库唯一索引是最终防重保障；Redis 不承担正确性。网络超时重试不会再次预占库存或优惠券。

## 库存和订单状态

`CommerceRepository.ReserveInventory` 使用带 `available >= quantity` 条件的一条 SQL 更新，并检查 `RowsAffected`。PostgreSQL 在行更新锁下判断条件；数据库检查约束还要求 `available >= 0`、`reserved >= 0`。支付成功扣减 `reserved`（已售），取消/超时把 `reserved` 还给 `available`。

| 原状态 | 允许的新状态 | 触发 |
|---|---|---|
| `PENDING_PAYMENT` | `PAID` | 已验签的成功回调 |
| `PENDING_PAYMENT` | `CANCELLED` | 订单本人调用取消接口 |
| `PENDING_PAYMENT` | `EXPIRED` | 超时 Worker |
| `PAID` | `PROCESSING` | 管理员推进 |
| `PROCESSING` | `COMPLETED` | 管理员推进 |

`internal/service/order_state.go` 的 `TransitionOrder` 统一拒绝其他转换（`409`），并返回与订单状态**同事务**保存的历史记录。`GET /api/orders/:id` 仅限本人；他人订单返回 `404`。

## Mock 支付与重复回调

`POST /api/orders/:id/payment` 为本人未过期的待支付订单创建或返回一笔 Mock Payment，**不会**扣款或直接标记已支付。`PaymentProvider` 和 `MockPaymentProvider` 位于 `internal/service/payment.go`。回调 `POST /api/payments/mock/webhook` 需要 `X-Mock-Signature`，即用 `MOCK_PAYMENT_WEBHOOK_SECRET` 对规范化事件字段计算 HMAC-SHA256；成功回调还校验订单号、支付引用、金额和订单状态。

`internal/service/payment_test.go` 的 `TestPaymentWebhookIsIdempotent` 展示真实签名调用 `provider.Sign(callback)`，同一事件发送十次仍只支付一次。`(provider, event_id)` 唯一约束和载荷哈希使相同回调可安全重放；同事件 ID 配不同内容返回 `409`。失败回调只标记支付尝试失败，订单仍待支付，可重试、取消或超时。接真实渠道前必须实现该渠道的验签、事件语义和对账规则。

## 超时与多实例

`cmd/server/main.go` 启动订单过期 Worker。它按 `ORDER_EXPIRATION_INTERVAL` 周期取批；PostgreSQL `FOR UPDATE SKIP LOCKED` 让多个实例领取不同订单。每笔订单在一个事务里转 `EXPIRED`、释放库存及优惠券、追加历史。状态锁避免并发取消/支付重复释放；优雅关闭时 Worker 随上下文停止。

## 验证规则

```bash
go test ./internal/service ./internal/repository
go test -race ./...
```

SQLite 测试覆盖回滚、优惠券、幂等、重复回调、状态转换；PostgreSQL 并发测试还需 `TEST_DATABASE_URL`，CI 会提供。`TestPostgresConcurrentOrdersNeverOversell` 用 100 个用户/购物车同时买库存 10，断言仅 10 单提交且库存不为负。测试接线见[扩展服务](/zh/development)，运维边界见[部署](/zh/deployment)。
