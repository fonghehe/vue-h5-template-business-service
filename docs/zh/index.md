---
layout: home

hero:
  name: "business-service"
  text: "vue-h5-template 的业务 API"
  tagline: 鉴权、用户资料与收藏、商品目录 —— 基于 Go、Gin 与 GORM，PostgreSQL 存储。
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/quickstart
    - theme: alt
      text: API 参考
      link: /zh/api
    - theme: alt
      text: 在 GitHub 上查看
      link: https://github.com/fonghehe/vue-h5-template-business-service

features:
  - title: 对齐前端契约
    details: 每个响应都是 <code>{ code, message, data, error, requestId }</code>，<code>code === 0</code> 表示成功，与 <code>@vh5/api-client</code> 完全一致。
  - title: 分层架构
    details: config → model → repository → service → httpapi，配合版本化迁移与确定性、幂等的种子数据。
  - title: 生产级健壮性
    details: 快速失败配置校验、按 IP 限流、CORS 与可信代理加固、请求关联 ID、结构化 JSON 日志。
  - title: 可运维
    details: 多阶段非 root Docker 镜像、健康/就绪探针、带 PostgreSQL 的 docker compose、GitHub Actions CI。
---

## 后端双服务

这是 vue-h5-template 后端的一半。流式 AI 工作负载位于姊妹仓库
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service)。两个服务共享 JWT 密钥与
响应信封，因此前端只需一个客户端。

| | 业务服务（Go） | AI 服务（Python） |
|---|---|---|
| 负载类型 | 短事务型 CRUD | 长连接流式 |
| 扩容维度 | 请求速率 | 并发流数量 |
| 故障模式 | 数据库延迟 | 上游模型延迟 |

## 演示账号

空数据库启动时会自动写入种子数据：

| 用户名 | 密码 | 角色 |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |
