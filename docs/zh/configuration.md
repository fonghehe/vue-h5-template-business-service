# 配置参考

Go 进程只读取环境变量，**不会自动加载** `.env`；本仓库也没有 `.env.development` / `.env.production` 加载器。`.env.example` 是模板。Docker Compose 的 `.env` 只参与 Compose 变量插值（这里主要是 `POSTGRES_PORT`），API 容器的环境变量在 `docker-compose.yml` 中显式配置。原生进程须在 Shell 中 export。`TEST_DATABASE_URL` 仅供 PostgreSQL 集成测试读取，不属于 `config.Load()`。

配置是**快速失败**的：非法或不安全的值会导致启动中止，而不是静默回退到默认值。

## 完整参考

| 变量 | 默认值 | 说明 |
|---|---|---|
| `APP_ENV` | `development` | `development` / `test` / `production` 之一。 |
| `PORT` | `8002` | HTTP 监听端口。 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` 之一。 |
| `LOG_FORMAT` | 非生产 `text`，生产 `json` | 生产必须为 `json`。 |
| `DATABASE_DRIVER` | `postgres` | `postgres` 或 `sqlite`（sqlite 仅用于隔离演示/测试）。 |
| `DATABASE_URL` | `postgres://…` | 按驱动指定的 DSN。 |
| `DB_MAX_OPEN_CONNS` | `25` | 最大打开连接数。 |
| `DB_MAX_IDLE_CONNS` | `5` | 连接池保留的最大空闲连接数。 |
| `DB_CONN_MAX_LIFETIME` | `30m` | 连接可被复用的时长。 |
| `AUTO_MIGRATE` | `true` | 启动时执行 schema 迁移。 |
| `SEED` | 非生产 `true`，生产 `false` | 开发写演示数据；生产拒绝 `true`。 |
| `JWT_SECRET` | dev 占位符 | **必须**与 AI 服务一致，至少 32 字符。 |
| `JWT_ISSUER` | `vue-h5-template` | `iss` 声明；必须与 AI 服务一致。 |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` 声明；必须与 AI 服务一致。 |
| `ACCESS_TOKEN_TTL` | `2h` | 访问令牌有效期。 |
| `REFRESH_TOKEN_TTL` | `336h` | 刷新令牌有效期（必须大于 `ACCESS_TOKEN_TTL`）。 |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | 承载刷新令牌的 HttpOnly cookie。 |
| `REFRESH_COOKIE_SECURE` | 非生产 `false`，生产 `true` | 标记刷新 cookie 为 `Secure`。 |
| `CORS_ORIGINS` | `http://localhost:5173,…` | 逗号分隔的浏览器源白名单。 |
| `RATE_LIMIT_ENABLED` | `true` | 开启按 IP 令牌桶限流。 |
| `RATE_LIMIT_RPS` | `20` | 每 IP 持续的每秒请求数。 |
| `RATE_LIMIT_BURST` | `40` | 每 IP 的桶容量。 |
| `ORDER_PAYMENT_TTL` | `15m` | 待支付订单预占期限。 |
| `ORDER_EXPIRATION_INTERVAL` | `30s` | 订单超时 Worker 间隔。 |
| `ORDER_EXPIRATION_BATCH_SIZE` | `100` | 每批最多领取订单数（上限 1000）。 |
| `MOCK_PAYMENT_WEBHOOK_SECRET` | 开发占位符 | Mock 回调 HMAC 密钥；生产必须更换且至少 32 字符。 |
| `REDIS_URL` | 空 | 可选的商品目录 Redis 缓存；为空即关闭。 |
| `PRODUCT_CACHE_TTL` | `5m` | 商品目录缓存寿命。 |
| `SHUTDOWN_TIMEOUT` | `15s` | 优雅关闭的时限。 |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP 服务器超时。 |

## 生产环境校验

生产默认 `SEED=false`，显式设为 `true` 也会拒绝启动，防止演示账号与优惠券进入正式环境。

当 `APP_ENV=production` 时，除非满足以下全部条件，否则启动会拒绝运行：

- `JWT_SECRET` 不是默认占位符，且至少 32 字符。
- `MOCK_PAYMENT_WEBHOOK_SECRET` 不是默认占位符，且至少 32 字符。
- `REFRESH_COOKIE_SECURE` 为 `true`。
- `CORS_ORIGINS` 列出显式源 —— 不能是 `*`，也不能是回环源（`localhost`、`127.0.0.1` 等）。
- `LOG_FORMAT` 为 `json`。

这是刻意的：一个配置错误却能启动的服务，远比一个启动即崩溃的服务危险得多。

## 隔离运行用 SQLite

`DATABASE_DRIVER=sqlite` 用于演示和隔离测试，此时无需完整 PostgreSQL。**不建议**用于生产环境。

```bash
DATABASE_DRIVER=sqlite DATABASE_URL=/tmp/vue-h5-business.db make run
```
