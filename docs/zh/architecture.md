# 架构说明

服务遵循严格的分层架构。依赖单向流动 —— 从传输层向下到数据库 —— 因此超脱 HTTP 的业务规则永远不会泄漏进
handler。

```
cmd/server          入口：装配、生命周期、优雅关闭
internal/config     快速失败的环境配置
internal/model      持久化实体（公开 JSON 契约）
internal/database   连接、版本化迁移、幂等种子
internal/repository 数据访问（GORM）
internal/service    业务规则 —— 唯一有权做决策的层
internal/httpapi    Gin 路由、handler、中间件
internal/response   共享的 JSON 信封
internal/apierr     稳定的应用错误码
internal/auth       JWT 签发 / 校验
internal/logging    结构化 JSON / 文本日志
```

## 请求生命周期

1. **中间件链** 分配请求 ID、设置安全头、捕获 panic、记录访问日志、应用 CORS、限制请求体大小。
2. **Handler** 只负责绑定输入，然后调用 service 方法。
3. **Service** 执行业务规则，失败时返回 `*apierr.Error`。
4. **Handler** 通过 `response.OK` / `response.Fail` 写回信封。
5. **未知路由** 返回同样的 JSON 信封 —— API 永远不会输出 HTML。

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
- **刷新** 用 cookie（或非浏览器客户端在 body 中提供的刷新令牌）轮换访问令牌。
- **AI 服务** 校验同样的访问令牌 —— `JWT_SECRET`、`JWT_ISSUER`、`JWT_AUDIENCE` 在两个服务间必须完全一致。

## 数据

- PostgreSQL 是生产数据库；SQLite 用于隔离测试与演示。
- Schema 由版本化迁移管理，`AUTO_MIGRATE` 开启时自动执行。
- 种子数据是幂等的 —— 只填充空表，因此绝不会覆盖真实数据。
- 金额以字符串存储，避免浮点漂移。
