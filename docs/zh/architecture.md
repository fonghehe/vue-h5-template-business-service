# 架构说明

这是单个 Go 进程：Gin 处理 HTTP，Service 执行业务规则，GORM Repository 访问 PostgreSQL。本仓库没有前端 `src/`、页面路由、组件、客户端 Store、BFF、WebSocket 或 SSE。Redis 只缓存商品目录；流式 AI 服务在独立仓库。

```
cmd/server          入口：装配、生命周期、优雅关闭
internal/config     快速失败的环境配置
internal/model      持久化实体（部分保留旧版 JSON 字段标签）
internal/database   连接、版本化迁移、幂等种子
internal/repository 数据访问（GORM）
internal/service    业务规则 —— 唯一有权做决策的层
internal/httpapi    Gin 路由、handler、中间件
internal/response   共享的 JSON 信封
internal/apierr     稳定的应用错误码
internal/auth       JWT 签发 / 校验
internal/logging    结构化 JSON / 文本日志
internal/cache      可选 Redis 商品目录 Cache Aside
internal/metrics    独立的 Prometheus 指标注册表
docs/               独立 VitePress 文档站（pnpm）
```

真实调用链是 `H5 客户端 → Gin 中间件/Handler → Service → Repository → PostgreSQL`。商品详情可命中 Redis 目录缓存，但库存仍从 PostgreSQL 实时读取。`cmd/server/main.go` 同时启动 HTTP 与订单过期 Worker，并负责优雅退出。

## 请求生命周期

1. **中间件链** 分配请求 ID、设置安全头、捕获 panic、记录访问日志和 Prometheus 指标、应用 CORS、限制请求体大小。启用时 `/api` 分组还有实例内按 IP 限流。
2. **Handler** 只负责绑定输入，然后调用 service 方法。
3. **Service** 执行业务规则，失败时返回 `*apierr.Error`。
4. **Handler** 通过 `response.OK` / `response.Fail` 写回信封。
5. **未知路由** 返回同样的 JSON 信封。受保护路由验证 JWT，管理路由还要求 `admin` 角色。

## 错误模型

每个失败都是 `apierr.Error`，携带三份相互独立的信息：

- **HTTP 状态码**（给客户端、负载均衡、可观测性工具）；
- 稳定的**应用错误码**（`4xxx` / `5xxx`），客户端代码可安全分支；
- **人类可读信息**，可安全展示给终端用户。

未知错误被归一化为不透明的 `5000` —— 原始原因会被记录但绝不泄漏给客户端。

## 鉴权流程

- **登录** 校验凭据（常量时间 bcrypt，未命中路径用假哈希摊平时序），随后签发访问令牌与刷新令牌。
  刷新令牌写入 `HttpOnly` cookie；访问令牌在响应体中返回。
- **访问令牌** 短期有效（`2h`），以 `Authorization: Bearer …` 发送。
- **刷新** 用 cookie（或非浏览器客户端在 body 中提供的刷新令牌）签发新令牌对。当前没有服务端会话/刷新令牌存储，也不支持即时撤销访问令牌：退出仅清除 cookie，客户端需丢弃 access token。
- 若其他服务需要接受本服务的 JWT，须在那里配置一致的 `JWT_SECRET`、`JWT_ISSUER`、`JWT_AUDIENCE`。本仓库不实现或验证独立的 AI 服务。

## 数据

- PostgreSQL 是运行时的数据真相；SQLite 只用于隔离测试与演示，无法验证 PostgreSQL 行锁语义。
- `internal/database/migrate.go` 在 `AUTO_MIGRATE=true` 时执行有记录的顺序迁移；独立数据迁移为旧商品补默认 SKU。`internal/database/seed.go` 仅在 `SEED=true` 时写入演示账号、目录和优惠券，不覆盖已有表数据。
- SKU、订单、支付金额使用 `int64` 分；旧 Product 的 `price` / `vipPrice` 字符串仅为 H5 兼容字段，不参与结算。

## 交易一致性

- 订单创建、库存预占、商品快照、优惠券预占、清空购物车在同一个事务提交或回滚。
- PostgreSQL 条件 `UPDATE available >= quantity` 防超卖；`(user_id, idempotency_key)` 唯一约束防重复订单。
- `(provider, event_id)` 唯一约束防重复支付回调；取消/超时在状态事务内释放库存和优惠券。
- 多实例过期 Worker 使用 `FOR UPDATE SKIP LOCKED`，订单更新还有版本校验。
- Redis 不缓存订单或库存；商品目录更新使缓存失效，详情库存每次直读 PostgreSQL。

具体流程见[交易闭环](/zh/commerce)，扩展路径见[扩展服务](/zh/development)。
