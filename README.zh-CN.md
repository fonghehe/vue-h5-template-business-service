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

仓库中的 Compose 配置会启动 API、PostgreSQL 和 Redis，并使用**仅供开发使用**的凭据与种子数据。源码开发时复制 `.env.example` 为项目根目录的 `.env`，程序会自动加载；已有进程环境变量优先。可运行的数据库地址与命令见[快速开始](https://fonghehe.github.io/vue-h5-template-business-service/zh/quickstart)。

如果要从源码启动 API，而不是运行 Compose 的 `business` 服务：

```bash
cp .env.example .env # 仅首次；按需修改项目内的 DATABASE_URL
POSTGRES_PORT=5433 docker compose up -d --wait postgres # 需要容器数据库时执行
go run ./cmd/server
```

示例 `.env` 使用 5433 端口的专用开发数据库。如果已有本地 PostgreSQL，在 `.env` 中配置本服务的专用数据库地址，即可省略 Compose 命令。缺少 `.env` 且未指定 `DATABASE_URL` 时，Go 命令仍会回退到 `localhost:5432`，那里可能属于其他应用。不要为了启动本服务而迁移或删除那个数据库。

## 交互式 API 文档

开发环境 Swagger 地址为 [http://localhost:8002/swagger/index.html](http://localhost:8002/swagger/index.html)。当前 OpenAPI 契约位于 `/openapi.yaml`。页面通过 `github.com/swaggo/gin-swagger` 注册，资源由 `github.com/swaggo/files` 提供，无需独立文档服务或 CDN。中间件直接读取现有 OpenAPI 3 契约，不需要用 `swag init` 另行生成 Swagger 2 文档。`APP_ENV=production` 时不注册这些路由。

通过 `POST /api/auth/login` 使用开发账号 `user` / `123456` 登录，复制 `data.accessToken` 到 **Authorize** 即可调试鉴权接口。管理接口使用 `admin` / `123456`。支付接口为模拟支付，不会真实扣款。

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

## 架构与生产运维

保留 `Handler → Service → Repository → PostgreSQL`，显式 HTTP DTO 固定客户端契约。`make migrate` 执行顺序迁移，`make migrate-status` 查询版本，不启动 HTTP 或写种子。staging/production 默认 `AUTO_MIGRATE=false`，启动副本前必须完成迁移。独立 `compose.production.yml` 使用外部提供的专用数据库部署镜像。部署前阅读[生产行为与升级说明](docs/zh/production.md)。刷新会话现已支持单次轮换与退出撤销，已有旧刷新令牌的用户需要重新登录。
