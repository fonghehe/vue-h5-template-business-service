# API 参考

业务 JSON 路由（包括错误与未知路由）返回同样的信封。`/metrics` 返回 Prometheus 文本，CORS 预检返回 HTTP 204，不是 JSON。

## 响应信封

```jsonc
// 成功
{ "code": 0, "message": "ok", "data": { "…": "…" }, "error": null, "requestId": "01J…" }

// 失败
{ "code": 4010, "message": "username or password is incorrect", "data": null, "error": null, "requestId": "01J…" }
```

前端以 `code === 0` 作为分支依据；HTTP 状态码反映错误类别，但应用层 `code` 才是权威信号。

## 错误码

| 码 | 含义 | HTTP |
|---|---|---|
| `0` | 成功 | 200 / 201 |
| `4000` | 请求错误 | 400 |
| `4001` | 校验失败 | 422 |
| `4010` | 未授权 | 401 |
| `4030` | 禁止访问 | 403 |
| `4040` | 未找到 | 404 |
| `4090` | 冲突 | 409 |
| `4130` | 载荷过大 | 413 |
| `4290` | 被限流 | 429 |
| `5000` | 内部错误 | 500 |
| `5030` | 服务不可用 | 503 |

## 鉴权

访问令牌是 JWT（`HS256`）。刷新令牌默认通过限定在 `/api/auth` 路径下的 `HttpOnly` cookie（`vh5_refresh`）下发，JavaScript 不能读取。退出只清除 cookie；已签发的无状态 access token 不会被服务端即时撤销。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| `POST` | `/api/auth/login` | — | 凭据换取访问令牌；写入刷新 cookie。 |
| `POST` | `/api/auth/refresh` | 刷新 cookie 或 body | 轮换过期的访问令牌。 |
| `POST` | `/api/auth/logout` | — | 清除刷新 cookie。 |

### POST /api/auth/login

**Body** `{ "username": string, "password": string }`

**响应 `data`**

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

## 用户

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| `GET` | `/api/user/info` | Bearer | 当前登录账号。 |
| `GET` | `/api/user/favorites` | Bearer | 收藏的商品，按时间倒序。 |

## 商品

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| `GET` | `/api/product/list` | — | 公开目录的一页。 |
| `GET` | `/api/product/detail?id=1` | — | 单个在售商品。 |
| `POST` | `/api/product/favorite` | Bearer | 收藏 / 取消收藏。 |

### GET /api/product/list

查询参数：`page`（默认 `1`）、`pageSize`（默认 `10`，最大 `50`）、`keyword`、`category`、`priceMin`、`priceMax`、`sort`（`newest`、`sales`、`price_asc`、`price_desc`，未知值回退到精选优先）。无效或非正分页值回退默认，超大页大小截为 50。价格是非负整数分，区间上下界须由同一个 active SKU 满足。

**响应 `data`** —— 分页信封：

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

旧 `price` / `vipPrice` 字符串用于 H5 展示兼容；真正结算价格是服务端重读的 SKU `int64` 分。

`GET /api/product/detail?id=<productId>` 返回在售 Product 及 `skus`（`skuCode`、`name`、JSON `attributes`、整数分 `price`/`originalPrice`、`status`）和当前 `inventory`（`available`、`reserved`、`version`）。即使目录字段命中 Redis，库存仍直读 PostgreSQL。公开列表不会预加载 SKU。

## 购物车、订单与支付

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` / `DELETE` | `/api/cart` | 查询 / 清空当前用户购物车。 |
| `POST` | `/api/cart/items` | 添加 SKU，数量限制 1–99；不接受客户端价格。 |
| `PUT` / `DELETE` | `/api/cart/items/:id` | 修改数量 / 删除本人购物车项。 |
| `GET` / `POST` | `/api/orders` | 查询本人订单 / 使用 `Idempotency-Key` 创建订单。 |
| `GET` | `/api/orders/:id` | 仅查询本人订单，越权统一返回 404。 |
| `POST` | `/api/orders/:id/cancel` | 取消待支付订单并释放库存、优惠券预占。 |
| `POST` | `/api/orders/:id/payment` | 为未过期的待支付订单创建或返回 Mock Payment；不真实扣款。 |
| `POST` | `/api/payments/mock/webhook` | HMAC 验签且数据库幂等的支付回调。 |

下单会在一个 PostgreSQL 事务中重新读取 SKU 服务端价格、校验优惠券、原子预占库存、创建订单快照、
预占优惠券并清空购物车。SKU、订单和支付金额均使用 `int64` 分。

添加购物车 body：`{"skuId":1,"quantity":2}`，它**设置**该 SKU 的数量，不是在旧数量上加 2；更新 body：`{"quantity":3}`。均要求 1–99、商品和 SKU 可售，结算时不同购物车项不能超过 100。

创建订单 body 为 `{}` 或 `{"couponCode":"WELCOME10"}`，必须带最长 128 字符的 `Idempotency-Key`。首次成功 HTTP 201；同 Key、同请求重试返回原订单 HTTP 200；同 Key 更换优惠券返回 409。订单列表使用 `page`/`pageSize`（最大 50）。

## Mock 支付回调

`POST /api/payments/mock/webhook` 的字段为 `eventId`、`orderNo`、`reference`、整数分 `amount`、`status`（`SUCCESS` / `FAILURE`）。`X-Mock-Signature` 是 `MockPaymentProvider.Sign` 规范字段的 HMAC；`internal/service/payment_test.go` 有真实签名用例。`(provider,eventId)` 数据库唯一约束保证相同事件不会重复支付；同 ID 不同内容返回 409。`FAILURE` 只让支付尝试失败，订单仍待支付直至重试、取消或超时。这不是实际支付平台。

## 管理端

运营路由需要 `admin` 角色，是唯一能修改商品的路由。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/admin/products` | 包含隐藏项的分页目录（支持 `status`、`featured` 过滤）。 |
| `POST` | `/api/admin/products` | 创建商品。 |
| `GET` | `/api/admin/products/:id` | 不限状态查询单个商品。 |
| `PATCH` | `/api/admin/products/:id` | 部分更新商品。 |
| `DELETE` | `/api/admin/products/:id` | 软删除商品。 |
| `POST` | `/api/admin/products/:id/skus` | 同事务创建 SKU 与初始库存。 |
| `PUT` | `/api/admin/skus/:id` | 更新 SKU。 |
| `PUT` | `/api/admin/orders/:id/status` | 推进 `PAID → PROCESSING → COMPLETED`。 |

### 商品载荷

`name`、`categoryId`、`brand`、`cover`、`title`、`imgUrl`、`price`、`vipPrice`、`shopDesc`、`delivery`、`shopName`、`description`、`stock`、`status`（`draft` | `on_sale` | `sold_out`）、`featured`。创建仍要求旧 `title`、`imgUrl`、字符串 `price`/`vipPrice` 和 `shopName`；`PATCH` 未传字段不变。`stock` 仅兼容旧字段，不是权威库存。

SKU 创建 body：`skuCode`、`name`、`attributes`（字符串 map）、`price`、`originalPrice`（整数分）、`status`（`active` | `inactive`）、`available`（初始库存）。SKU PUT 校验同样字段，但不会直接修改库存可用量。目前没有公开的优惠券管理接口。事务细节见[交易闭环](/zh/commerce)。

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/health` | 存活探针；不触碰数据库。 |
| `GET` | `/ready` | 就绪探针；数据库不可达时失败。 |
| `GET` | `/metrics` | Prometheus HTTP 与订单/库存业务指标。 |
| `GET` | `/api/health` | 兼容存活接口。 |
