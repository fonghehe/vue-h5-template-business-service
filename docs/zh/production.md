# 生产行为与升级说明

项目保留分层 Go 单体。Handler 负责协议、绑定与 DTO 映射，Service 控制业务规则和事务，Repository 执行带 Context 的 SQL。流式 AI 保持在独立服务。旧 `/api` 和 JSON 字段名继续兼容，`/api/v1` 注册同一组业务处理器，两套路径共用限流器。刷新 cookie 作用域与登录时使用的认证路径一致。

## 升级已有安装

1. 备份本服务的专用 PostgreSQL 数据库，并在恢复副本中演练。
2. 使用新版本及审核过的环境配置运行 `make migrate-status`、`make migrate`。
3. 新增的顺序迁移创建 `refresh_sessions`，补上交易约束与索引。已有非法记录会使事务失败，应排查记录而非关闭约束。PostgreSQL 事务级 advisory lock 按数据库/schema 串行化迁移执行者；整批待执行迁移失败时回滚。没有自动破坏性 down：使用已验证的备份恢复或经审核的前向修复。
4. 副本以 `AUTO_MIGRATE=false`、`SEED=false` 启动，缺少迁移版本就拒绝提供 HTTP。staging/production 默认如此。开发环境仍可启动迁移和种子；种子有互斥且只填充空表。
5. 旧 Access JWT 在账号有效时继续可用；旧 Refresh JWT 没有会话记录，用户需重新登录。应与前端发布协调。

迁移标识使用顺序编号和语义名称（例如 `004_extend_product_catalogue`），不使用日期。已有数据库中的日期标识仍被识别为已执行，无需手动改写账本；新执行的迁移写入新标识。后续变更只追加迁移，不复用或修改已发布步骤。

## 认证与浏览器边界

PostgreSQL 只保存刷新 JWT 的 SHA-256 摘要。刷新在同一事务中消费一个有效会话并创建替代会话，并发使用只有一个成功者。退出撤销所提交的刷新令牌并清除 cookie；Access JWT 仍为无状态令牌，在到期前有效，客户端必须丢弃。本实现是当前会话退出，不是全设备退出，也没有令牌家族失窃检测。被盗的新令牌仍可能在撤销/过期前使用。

每次鉴权业务请求读取当前账号状态与角色，禁用账号、移除 admin 在本服务下一次请求生效。独立 AI 服务需要自己的对应策略。当前 `user` / `admin` 角色适合已有接口，未新增动态权限表、权限配置后台或权限管理 UI。

浏览器需携带 cookie credentials，并将刷新请求串行化。只接受配置的浏览器 Origin；无 Origin 的请求支持非浏览器客户端，CORS 不能替代鉴权。刷新/退出可选 JSON 字段 `refreshToken`；即使使用 cookie，非法或尾随 JSON 也返回 422。

## 资源与运行限制

- `REQUEST_TIMEOUT=10s` 将协作式 deadline 传给 SQL/缓存，超时返回 HTTP 504 / code 5040，且必须小于 `WRITE_TIMEOUT`。它不能强行停止忽略 Context 的 CPU 工作，也不能保证取消回滚已提交事务；结算重试应复用幂等 Key。
- 请求体上限 1 MiB，包含分块传输及尾随数据；分页上限保持 50。实例限流器最多保留 10,000 个客户端桶，满容量时拒绝新客户端，待闲置记录过期后恢复。认证接口额外共用每秒 0.5 次、突发 5 次的限制。多副本需边缘共享防护，本服务不信任任意转发 IP 请求头。
- `/metrics` 包括 HTTP 次数/耗时、业务计数、数据库连接池和 Go 运行时指标；进程指标取决于平台支持。接口无 JWT，应限制在内网或代理层。未配置 OpenTelemetry 导出和追踪后端。
- SQL 错误/慢查询时长写结构化日志并带 requestId，不记录原始 SQL 与绑定参数。请求元数据通过不依赖 Gin 的 Context 工具传递。Worker 有限批次清理过期会话、逐单回收超时订单并响应取消；收到信号时取消 Worker，HTTP 排空后等待其退出，再关闭缓存和数据库。
- Redis 仍是可选目录缓存。连接、池等待和命令等待有上限，尊重请求 deadline，禁用重试并增加 TTL 抖动。库存实时读 PostgreSQL。失效是尽力而为，故障或并发回填后目录可能保持旧值至 TTL 到期。就绪仅依赖 PostgreSQL，因为 Redis 可降级。

## 运行审核过的生产镜像

默认 `docker-compose.yml` 保持本地开发用途，独立生产文件要求环境配置及已有专用数据库：

```bash
docker build -t vue-h5-template-business-service:local .
# 部署环境必须提供 DATABASE_URL、JWT_SECRET、
# MOCK_PAYMENT_WEBHOOK_SECRET、CORS_ORIGINS，禁止沿用开发密钥。
docker compose -f compose.production.yml run --rm migrate
docker compose -f compose.production.yml up -d business
```

宿主端口绑定 localhost，进程非 root、文件系统只读并移除 capabilities，需要上游 TLS 代理。这里不创建生产用户、证书、数据库备份或指标存储。镜像含 `/app/server` 和 `/app/migrate`，探针分别使用 `/health` 与 `/ready`。

## 商业能力边界

支付提供方明确是模拟器，不会进行真实资金结算。真实支付上线仍需渠道验签、避免在长数据库事务中调用外部平台的幂等支付创建、回调重试策略、对账和退款流程。没有业务需求时不引入 MQ、Cron 平台或闲置外部 Client 抽象。
