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
cp .env.example .env
docker compose up --build
```

这会启动 API 加上带持久化卷与健康检查的 PostgreSQL 17。API 监听 `http://localhost:8002`。

## 生产环境检查清单

- **替换所有密钥** —— `JWT_SECRET`（至少 32 字符、随机）、PostgreSQL 密码。绝不要使用默认值。
- **设置 `APP_ENV=production`** —— 这会打开下面的快速失败校验。
- **在应用上游终结 TLS**（nginx、负载均衡或托管网关），并设置 `REFRESH_COOKIE_SECURE=true`。
- **锁定 `CORS_ORIGINS`** 为真实前端域名。生产启动会拒绝 `*` 与回环源。
- **使用 JSON 日志**（`LOG_FORMAT=json`）并接入采集器。
- **保持 `JWT_SECRET`/`JWT_ISSUER`/`JWT_AUDIENCE` 与 AI 服务一致**，使此处签发的令牌能在彼处校验。
- **存活/就绪探针** 分别指向 `/health` 与 `/ready`。

## 优雅关闭

收到 `SIGINT`/`SIGTERM` 后，服务器停止接受新连接，在 `SHUTDOWN_TIMEOUT` 内排空在途请求，然后关闭数据库
连接池并退出。

## CI

GitHub Actions 在每个 PR 与 `main` 的 push 上运行：格式检查、`go vet`、`go test -race` 及覆盖率、
`golangci-lint`，以及针对 PostgreSQL service 容器的 Docker 镜像构建。
