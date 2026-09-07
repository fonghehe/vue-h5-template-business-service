# 快速开始

启动 Go 业务 API。本仓库不包含 H5 页面；`docs/` 是独立的 VitePress 文档项目。

## 前置条件

- 完整本地栈使用 **Docker Compose**；源码运行需要 **Go 1.25+** 和可用的 PostgreSQL。
- 编辑或构建文档才需要 **Node.js 22+** 与 **pnpm 11**。

## 方式 A —— Docker Compose

现有 Compose 同时启动 API、PostgreSQL 17 和 Redis 8。开发默认执行迁移并写入演示账号、商品、SKU 与优惠券。Compose 已显式设置服务环境变量；复制 `.env.example` 不会配置其中的 JWT、数据库 URL 或 Redis URL。

```bash
docker compose up -d --build
```

API 监听在 `http://localhost:8002`。验证一下：

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
docker compose ps
```

## 方式 B —— 从源码运行

```bash
# 仅启动 PostgreSQL；宿主机使用 5433，避开现有的 5432 服务。
POSTGRES_PORT=5433 docker compose up -d postgres
export DATABASE_URL='postgres://vue_h5:vue_h5_local@127.0.0.1:5433/vue_h5_business?sslmode=disable'
go run ./cmd/server
```

`make run` 执行相同命令。不要同时启动 Compose 的 `business` 服务，它也占用宿主机 8002 端口。Go 进程**不会自动加载 `.env`**，请在 Shell 导出变量或使用 env-file 工具。`.env.example` 包含 5432 端口的开发凭据，使用上述命令时需把端口改成 5433。原生 API 不用 Redis 时留空 `REDIS_URL`。如果默认 Go 模块代理超时，可先运行 `GOPROXY=https://goproxy.cn go mod download`，并保持仓库的 `go.sum` 校验开启。详见[配置参考](/zh/configuration)。

不设置 `DATABASE_URL` 时，裸跑 `go run ./cmd/server` 会连接默认的 `localhost:5432`，那里可能是其他应用的数据库。**不要**为了启动本服务去迁移或删除其表；请使用上面的专用 Compose 数据库与导出的连接地址。启动时会在应用迁移前拒绝不兼容的 `users`/`products` 表结构。

## 试用 API

用种子账号登录并读取商品目录：

```bash
# 1. 登录（返回 accessToken，同时写入 refresh cookie）
curl -s http://localhost:8002/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"user","password":"123456"}'

# 2. 用上一步的 token 读取个人资料
curl -s http://localhost:8002/api/user/info \
  -H 'Authorization: Bearer <accessToken>'

# 3. 收藏一个商品
curl -s http://localhost:8002/api/product/favorite \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"productId":1,"favorite":true}'
```

完整结算流程见[交易闭环](/zh/commerce)，接口明细见 [API 参考](/zh/api)。

## 开发循环

```bash
make check     # 格式化 + vet + 测试
make test-race # 竞态检测 + 覆盖率
make lint      # golangci-lint
make build     # 编译到 bin/server

cd docs
pnpm install --frozen-lockfile
pnpm docs:dev
pnpm docs:build
```

## 下一步

- [配置参考](/zh/configuration) —— 全部环境变量。
- [架构说明](/zh/architecture) —— 各分层如何协作。
- [部署指南](/zh/deployment) —— 上线到生产。
- [扩展服务](/zh/development) —— 新增接口、模型和测试。
