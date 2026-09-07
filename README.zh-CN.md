# vue-h5-template-business-service

[English](./README.md) | 简体中文 | [日本語](./README.ja.md)

面向 vue-h5-template 的 Go/Gin 商业 API。本仓库负责登录认证、用户资料、收藏、商品目录、SKU 库存、购物车、优惠券、订单和模拟支付状态。H5 界面与流式 AI 服务位于其他仓库。

核心交易链路为 `商品 → SKU → 库存 → 购物车 → 优惠券 → 订单 → 支付 → 取消/超时`。PostgreSQL 是数据真相：创建订单、预留库存与优惠券、保存订单快照、清空购物车在同一事务中完成。数据库唯一约束保障订单重试与支付回调的幂等性；过期任务释放未支付订单的预留资源。可选 Redis 仅缓存商品目录，不保存权威库存或订单数据。

## 技术栈

Go 1.25、Gin、GORM、PostgreSQL 17、可选 Redis、JWT、Prometheus，以及 VitePress 文档。SQLite 只用于隔离测试，不用于生产环境。支付采用带签名的**模拟支付提供方**，未接入真实支付平台。

## 快速启动

```bash
docker compose up -d --build
curl http://localhost:8002/health
curl http://localhost:8002/ready
curl 'http://localhost:8002/api/product/list?page=1&pageSize=10'
```

仓库中的 Compose 配置会启动 API、PostgreSQL 和 Redis，并使用**仅供开发使用**的凭据与种子数据。原生运行 Go 进程时必须显式导出环境变量；程序不会自动读取 `.env`。可运行的数据库地址与命令见[快速开始](https://fonghehe.github.io/vue-h5-template-business-service/zh/quickstart)。

如果要从源码启动 API，而不是运行 Compose 的 `business` 服务：

```bash
POSTGRES_PORT=5433 docker compose up -d postgres
export DATABASE_URL='postgres://vue_h5:vue_h5_local@127.0.0.1:5433/vue_h5_business?sslmode=disable'
go run ./cmd/server
```

裸跑 Go 命令会使用默认的 `localhost:5432`，那里可能属于其他应用。不要为了启动本服务而迁移或删除那个数据库。

## 检查

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
```

PostgreSQL 的“100 人抢购、库存 10 件”并发测试需要 `TEST_DATABASE_URL`；CI 会提供隔离测试数据库。详细文档提供三种语言：

- [English documentation](https://fonghehe.github.io/vue-h5-template-business-service/)
- [中文文档](https://fonghehe.github.io/vue-h5-template-business-service/zh/)
- [日本語ドキュメント](https://fonghehe.github.io/vue-h5-template-business-service/ja/)

本地运行文档：`cd docs && pnpm install --frozen-lockfile && pnpm docs:dev`。

## 许可证

[MIT](./LICENSE)
