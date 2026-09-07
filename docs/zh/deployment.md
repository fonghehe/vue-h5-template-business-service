# 部署指南

## 构建镜像

镜像采用多阶段构建，以**非 root** 用户运行，并在编译期注入版本号。

```bash
docker build --build-arg VERSION=$(git describe --tags --always --dirty) \
  -t vue-h5-template-business-service:latest .
```

或用 make：

```bash
make docker
```

## 用 docker compose 运行

```bash
docker compose up -d --build
```

现有 Compose 是**开发栈**：API、带持久化卷的 PostgreSQL 17、仅作缓存且不持久化的 Redis 8。它写死开发凭据和 `APP_ENV=development`；复制 `.env.example` 不会使其变成生产部署。API 位于 `http://localhost:8002`。`POSTGRES_PORT=5433 docker compose up -d --build` 只改变数据库宿主端口。

## 生产环境检查清单

- **替换所有密钥** —— `JWT_SECRET`、`MOCK_PAYMENT_WEBHOOK_SECRET`（均至少 32 字符）及 PostgreSQL 凭据，不要使用 Compose 默认值。
- **设置 `APP_ENV=production`** —— 这会打开下面的快速失败校验。
- **在应用上游终结 TLS**（nginx、负载均衡或托管网关），并设置 `REFRESH_COOKIE_SECURE=true`。
- **锁定 `CORS_ORIGINS`** 为真实前端域名。生产启动会拒绝 `*` 与回环源。
- **使用 JSON 日志**（`LOG_FORMAT=json`）并接入采集器。
- **关闭演示数据**（`SEED=false`）；生产会拒绝 `true`。真实用户/优惠券需通过经批准的运维流程准备；当前没有公开注册或优惠券管理接口。
- **协调迁移**：`AUTO_MIGRATE=true` 在启动时执行有记录的迁移。多副本应指定一个迁移执行者或提前迁移。旧 Product 即使关闭 Seed 也会经数据迁移获得默认 SKU/库存。
- **PostgreSQL 是数据真相**。Redis 仅是可选目录缓存；实例内按 IP 限流不会跨副本共享，多副本请在边缘配置共享限流。
- **按需限制 `/metrics`**：路由无 JWT；它提供 HTTP 数量/耗时、订单创建/支付/过期和库存预占失败指标。
- **支付只是 Mock**：HMAC 回调不能结算真实资金；接入真实渠道需重新设计验签、事件语义和对账。
- **保持 `JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` 与 AI 服务一致**，使此处签发的令牌能在彼处校验。
- **存活/就绪探针** 分别指向 `/health` 与 `/ready`。

## 优雅关闭

收到 `SIGINT`/`SIGTERM` 后，服务器停止接受新连接，在 `SHUTDOWN_TIMEOUT` 内排空在途请求，然后关闭数据库
连接池并退出；过期 Worker 和缓存客户端也会停止。替换实例启动后仍会处理待超时订单，PostgreSQL `SKIP LOCKED` 防止多副本重复领取。

## CI

GitHub Actions 在每个 PR 与 `main` 的 push 上运行：格式检查、`go vet`、`go test -race` 及覆盖率、
`golangci-lint`，以及使用 PostgreSQL service 容器的并发测试和 Docker 镜像构建。独立的 `deploy-docs.yml` 使用 pnpm 校验配置、构建三语 VitePress 并上传 GitHub Pages；构建成功不等于 Pages 已启用或公网可访问。
