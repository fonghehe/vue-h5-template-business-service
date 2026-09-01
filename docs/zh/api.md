# API 参考

业务服务暴露 JSON HTTP API。每个端点 —— 包括错误与未知路由 —— 都返回同样的信封。

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

访问令牌是 JWT（`HS256`）。刷新令牌通过限定在 auth 路径下的 `HttpOnly` cookie（`vh5_refresh`）下发，
JavaScript 永远读不到它。

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

查询参数：`page`（默认 `1`）、`pageSize`（默认 `10`，最大 `50`）、`keyword`、`sort`。

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

金额字段（`price`、`vipPrice`）以**字符串**存储和返回，避免数据库与客户端之间出现浮点舍入。

## 管理端

运营路由需要 `admin` 角色，是唯一能修改商品的路由。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/admin/products` | 包含隐藏项的分页目录（支持 `status`、`featured` 过滤）。 |
| `POST` | `/api/admin/products` | 创建商品。 |
| `GET` | `/api/admin/products/:id` | 不限状态查询单个商品。 |
| `PATCH` | `/api/admin/products/:id` | 部分更新商品。 |
| `DELETE` | `/api/admin/products/:id` | 软删除商品。 |

### 商品载荷

`title`、`imgUrl`、`price`、`vipPrice`、`shopDesc`、`delivery`、`shopName`、`description`、`stock`、`status`
（`draft` | `on_sale` | `sold_out`）、`featured`。`PATCH` 时未传字段保持不变。

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/health` | 存活探针；不触碰数据库。 |
| `GET` | `/ready` | 就绪探针；数据库不可达时失败。 |
