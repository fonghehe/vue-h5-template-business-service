# Gin 服务审计与渐进改造报告

审计日期：2026-10-02。基线为任务开始时的工作区，包含已有暂存改造；本轮保留这些修改，没有重新创建项目、删除业务模块或提交/推送代码。本报告描述当前实现与验证边界，生产升级步骤见[生产行为与升级说明](/zh/production)。

## 当前架构与已具备能力

项目是 Go 1.25 + Gin + GORM 的分层单体。PostgreSQL 是运行时数据库，SQLite 仅用于隔离测试，可选 Redis 为商品目录缓存。依赖还包括 JWT v5、bcrypt、Prometheus、Swagger UI，以及文档用的 VitePress。没有业务所需的 MQ、Cron、文件上传或真实第三方 HTTP 客户端，因此没有创建占位模块。

实际目录：

```text
cmd/server            HTTP 启动、信号处理、资源关闭
cmd/migrate           独立迁移 up/status
internal/httpapi       Gin 路由、请求绑定、显式响应 DTO
internal/service       认证、目录、收藏、购物车、优惠券、订单、支付规则
internal/repository    GORM 查询及绑定事务的 Repository
internal/database      连接池、版本迁移、开发种子
internal/model         持久化实体，JSON 标签仅保留缓存兼容用途
internal/config        dotenv + 环境变量 + 默认值及快速失败校验
internal/auth          JWT 签发与验证
internal/apierr        稳定业务错误及内部 Cause
internal/response      code/message/data/error/requestId 信封
internal/requestctx    不依赖 Gin 的请求元数据
internal/logging       slog 与 SQL 错误/慢查询日志
internal/cache         可降级的 Redis 目录缓存
internal/metrics       HTTP、业务、Go 运行时和连接池指标
docs                   英文、中文、日文 VitePress 文档
```

HTTP 调用链保持 `Router → Middleware → Handler → Service → Repository → PostgreSQL`，下层使用 `context.Context`。测试采用包内 `*_test.go`，未机械新增空的 `pkg/configs/scripts/tests/migrations` 目录；迁移仍由 `internal/database` 的版本账本执行。

原有值得保留的能力包括：统一信封和业务错误、JWT/bcrypt、管理员角色限制、分页上限与排序白名单、SQL 参数绑定、商品软删除、整数分金额、购物车所有者行锁、事务内库存条件更新、订单快照、优惠券预占、订单/回调唯一约束、订单版本校验、逐单 SKIP LOCKED 过期处理、可停止 Worker、独立指标注册表、非 root 多阶段镜像及 GitHub CI。

## 分级审计结果

| 级别 | 基线问题与实际影响 | 证据/当前处理 |
|---|---|---|
| P0 上线边界 | Mock 支付不具备真实资金结算能力；商业收款不能直接上线 | `internal/service/payment.go`；保留明确模拟器标识，真实渠道接入仍需外部资料 |
| P1 | Refresh 可重放；退出仅清除客户端 cookie | `service/service.go`、`repository/session.go`：摘要会话、原子消费/替换、退出撤销、过期清理 |
| P1 | JWT 中的旧 admin 角色/账号状态持续有效 | `httpapi/middleware/auth.go`：鉴权加载当前账号角色和状态 |
| P1 | 多实例迁移检查/写入存在竞争；缺少独立运维入口 | `database/migrate.go`、`cmd/migrate`：事务级锁、up/status、启动版本校验 |
| P1 | Request 没有业务 deadline；认证 CPU 消耗与限流状态容量未充分约束 | `middleware/middleware.go`：deadline、认证限流、10,000 桶容量上限 |
| P1 | 可选 JSON 解析吞错；分块/尾随数据可能绕过早停 JSON 解码器的大小判断 | `httpapi/helpers.go`：恰好一个 JSON 值；BodyLimit 读取有界内容并覆盖分块请求 |
| P1 | Model 直接输出；PATCH 在 Handler 合并旧数据，可能覆盖并发更新 | `httpapi/dto.go`、`service/product_patch.go`：显式 DTO 与行锁内合并、列白名单更新 |
| P2 | Context Deadline 被当作客户端断开，缺少明确超时错误 | `apierr/errors.go`：504 / 5040；取消请求不写伪成功响应 |
| P2 | panic 后访问日志/HTTP 指标可能没有执行 | 中间件顺序调整并增加 panic 后 500 日志/指标测试 |
| P2 | 缺少 Go 运行时与连接池指标；SQL 日志缺乏结构化关联 | `metrics/metrics.go`、`logging/database.go`：补 collector 与 requestId，省略原始 SQL/绑定参数 |
| P2 | 不支持 local/staging，CORS 回环地址识别不完整 | `config/config.go`：URL/IP 解析、完整回环/未指定地址判断与 staging 安全默认值 |
| P2 | 种子修改全局商品切片，且并行开发副本可能竞争初始化 | `database/seed.go`：复制种子切片、事务与 schema 专属互斥 |
| P3 | 动态权限管理、OTel 导出、真实支付/退款/对账、全设备退出/令牌家族检测 | 未为当前简单角色系统增加无业务价值的抽象；见后续建议 |

这里没有把 Mock 支付边界包装成已验证的漏洞，也不把 SQLite 测试等同于 PostgreSQL 行锁验证。源码审查未发现需要重写全系统才能解决的架构问题。

## 改造计划与实际变化

按配置/迁移基础、认证与 HTTP 边界、DTO/事务、部署/文档、最终验证的顺序实施。每批修改后编译并执行相关测试，失败修复后再推进。

- 新增：迁移命令、刷新会话实体与 Repository 操作、显式响应 DTO、认证测试替换接口、请求元数据工具、SQL 结构化日志、生产 Compose 文件和三语升级说明。
- 修改：配置默认值与校验、中间件、认证服务、商品 PATCH 事务、缓存超时/重试/TTL、指标、生命周期关闭顺序、Dockerfile、Makefile、CI 与前端 OpenAPI。
- 删除/移动：仅移除 Handler 内的 PATCH 业务合并和吞掉可选 JSON 错误的逻辑；没有删除业务文件，也没有机械迁移到 DDD/Clean Architecture。
- 依赖：继续使用已有栈，没有新增 MQ、Tracing SDK、配置框架或大型架构库；使用已存在的 Prometheus collector 和标准库。
- 数据库：追加 `008_create_refresh_sessions`、`009_enforce_commerce_constraints`。迁移标识统一使用顺序编号和语义名称，记录在数据库账本；兼容已有日期标识，不改写旧记录或重复执行。迁移整批使用事务锁，可重复执行；不提供自动删表 down。
- API：原 JSON 字段、旧 `/api` 路径和分页模型保留，新增 `/api/v1` 同行为别名。超时增加 5040，非法可选 JSON 变为 422；旧 Refresh JWT 因无会话记录需要重新登录。退出仅撤销当前刷新会话，Access JWT 仍到期失效。
- 契约：服务端 Swagger/OpenAPI 覆盖注册路由并验证本地引用；对应 `frontend/vue/turborepo-demo/vue-h5-template` 同步真实接口并重新生成客户端类型，保留 AI 操作和已有 schema。

## 安全与资源处理

鉴权不再完全相信令牌中的角色；账号禁用与角色撤销在下一次业务请求生效。刷新记录保存 SHA-256 摘要，数据库事务保证只消费一次，退出撤销会话。JWT 限定 HS256、issuer、audience、期限与令牌类型，拒绝非法/零 subject。密码仍使用 bcrypt。

浏览器 Origin 白名单、SameSite/HttpOnly cookie、生产 Secure cookie、Body 上限、分页上限、角色限制和按用户过滤的购物车/订单查询共同保护当前 HTTP 边界。无上传/用户文件路径入口，没有为了清单创建文件上传安全模块。JSON 内容的 HTML 转义仍是消费客户端的职责。

请求 Context 贯穿数据库和缓存；Worker 请求也有时间上限和停止/等待流程。slog 访问/业务/SQL 日志包含关联 ID，响应不序列化内部 Cause。Redis 尊重 Context、禁用自动重试、限制池等待并为 TTL 加抖动，库存不进入目录缓存。

限制仍明确：实例内 IP 限流不跨副本共享；请求 deadline 是协作式取消；Redis 失效是尽力而为；`/metrics` 需要内网或上游限制；独立 AI 服务不会自动获得此处新增的账号状态检查。

## 验证结果

本机检查不能当作公网部署或真实支付验收。PostgreSQL 测试使用独立 schema，容器启动验证同样使用专属临时 schema，不迁移或删除已有应用数据。

| 检查 | 结果与边界 |
|---|---|
| gofmt / go fmt | 格式检查通过 |
| go mod verify / go mod tidy -diff | 全部模块校验通过；tidy 无差异 |
| go vet ./... / go build ./... | 通过 |
| go test ./... | 通过 |
| go test -race ./... + TEST_DATABASE_URL | 通过，实际 PostgreSQL 并发测试未跳过 |
| golangci-lint run ./... | 通过；TTL jitter 的弱随机告警明确标注为非安全用途，其余检查保留 |
| docker build | 多阶段镜像构建通过，包含 server 与 migrate |
| 临时 schema 容器验收 | migrate status/up、/health、/ready、/api/v1/product/list、/openapi.yaml、/metrics 均通过；UID=10001；SIGTERM 退出码 0 |
| 前端 | api:generate 通过，全工作区 typecheck 通过；API 客户端 2 个测试文件 / 16 个测试通过 |
| 三语 VitePress | docs:typecheck、docs:build 通过 |

代表性行为验证包括：100 用户争抢 10 件库存、同用户并发结算仅消费一次购物车、结算期间新增购物车项保留、支付等锁后重查超时、过期失败不回滚其他订单、10 个并发刷新只有一个成功、多个迁移执行者串行化、退出/刷新重放拒绝、旧 JWT 角色撤销/账号禁用、deadline 504、分块超大 Body 413、panic 后 500 日志/指标与有界会话清理。

新增清理测试发现 SQLite 的本地时间与 UTC 字符串比较差异；会话到期值已统一 UTC，修复后完整测试/race 通过。客户端生成契约与当前源码一致，但没有把该检查扩大解释为前端全流程 E2E、真实 Redis 故障演练、外网部署或依赖漏洞数据库扫描。

## 未实现及后续建议

1. **真实支付**：当前没有渠道资料与真实交易环境。接入时实现渠道签名、外部意图幂等、回调重试、对账/退款；避免把网络请求放在长数据库事务里。
2. **实际生产运维**：用户/优惠券准备、TLS、备份恢复演练、监控存储和共享边缘限流取决于部署环境；仓库提供执行入口和约束，不擅自发布。
3. **按需求演进**：动态权限表、OTel 导出、全设备会话撤销和令牌家族检测需要产品/运维决策；目前仍使用简单角色模型。新增外部服务或可靠异步业务后，再引入具体 Client/Worker/outbox，而非提前创建空目录。
