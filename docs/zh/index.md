---
layout: home

hero:
  name: "business-service"
  text: "vue-h5-template 的商业服务 API"
  tagline: 一个 Go 服务完成认证、SKU 库存、购物车、优惠券与事务化订单。
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
  - title: 事务化结算
    details: 服务端 SKU 价格、条件库存预占、优惠券限制、订单快照与清空购物车在同一事务中提交。
  - title: 可安全重试的支付状态
    details: 数据库订单与回调幂等、明确的状态转换、取消及超时释放预占资源。
  - title: 兼容已有 H5 契约
    details: 认证、资料、收藏与商品接口保留统一 JSON 响应信封。
  - title: 可运维服务
    details: PostgreSQL 为数据真相，可选 Redis 商品缓存、结构化日志、Prometheus 指标和多实例过期 Worker。
---

## 后端双服务

本仓库是 Go HTTP 服务，不是 H5 前端；这里没有页面、Vue 组件、客户端 Store 或 Vite 应用。流式 AI 工作负载位于姊妹仓库
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service)。两个服务共享 JWT 密钥与
响应信封；前端分别路由两类请求。

| | 业务服务（Go） | AI 服务（Python） |
|---|---|---|
| 负载类型 | 事务化交易与账号数据 | 长连接流式 |
| 扩容维度 | 请求速率 | 并发流数量 |
| 故障模式 | 数据库延迟 | 上游模型延迟 |

## 演示账号

仅 `SEED=true` 的开发环境写入演示账号；生产环境禁止启用。不要在生产使用这些凭据：

| 用户名 | 密码 | 角色 |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |

从[快速开始](/zh/quickstart)启动服务，在[交易闭环](/zh/commerce)理解一次结算，再用[扩展服务](/zh/development)添加接口或模型。
