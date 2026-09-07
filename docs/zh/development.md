# 扩展服务

本仓库是**后端**。新增 H5 页面、路由菜单、Vue 组件、Composable、客户端 Store、主题或前端国际化，应在独立的前端仓库完成。这里对应的开发工作是新增 Go HTTP 接口或扩展业务事务。

## 先沿着现有接口读代码

`GET /api/cart` 是一个真实、简短的鉴权查询例子：

1. `internal/httpapi/router.go`：`cart.GET("", s.getCart)` 位于使用 `middleware.Authenticate` 的分组中。
2. `internal/httpapi/handlers_commerce.go`：`getCart` 从 `middleware.CurrentUserID(c)` 取本人 ID，调用 `s.services.Cart.List`，用 `response.OK` / `response.Fail` 返回。
3. `internal/service/commerce_service.go`：`CartService.List` 委托 Repository；同文件里的写操作负责数量、可售状态与订单状态规则。
4. `internal/repository/commerce.go`：`ListCart` 在 SQL 中 `Where("user_id = ?", userID)`，并预加载 SKU/Product。归属限制要在查询层执行，而非仅在 Handler 中隐藏字段。
5. `internal/httpapi/router_test.go` 与 `internal/service/commerce_test.go` 分别覆盖 HTTP 契约和业务规则。

下面的 Handler 来自现有代码，只省略了周围声明：

```go
func (s *Server) getCart(c *gin.Context) {
    items, err := s.services.Cart.List(c.Request.Context(), middleware.CurrentUserID(c))
    if err != nil {
        response.Fail(c, err)
        return
    }
    response.OK(c, gin.H{"items": items})
}
```

新增查询时，依次在 `router.go` 注册路由、在 `internal/httpapi` 写小型校验与响应 Handler、在 `internal/service` 写持久业务规则、在 `internal/repository` 写 SQL，最后补 Router 和 Service 测试。需鉴权/管理员的路由应加入现有对应分组。用户身份、结算价格、订单状态等权威数据不可从客户端直接信任。SQL 参数必须绑定，不要拼接请求字符串。

## 新增字段或持久化实体

1. 在 `internal/model` 定义持久化字段及 JSON 名称；已有公开 JSON 名称涉及兼容性。
2. 在 `internal/database/migrate.go` **追加**版本迁移，不要修改已发布迁移。`internal/database/seed.go` 仅用于确定性的开发数据；生产迁移不能依赖 `SEED=true`。
3. 按不变量与查询模式添加 GORM 索引/约束。交易金额用 `int64` 分，不用 float；保留历史 OrderItem 快照。
4. 在 Repository 增加数据操作。多次写入用 `CommerceRepository.Transaction(ctx, fn)`；库存复用条件更新，订单状态复用 `TransitionOrder`，不要在 Handler 直接赋状态。
5. 公共 API 变化时，同步三种语言的接口文档及前端仓库的 OpenAPI 契约。

`internal/database/migrate_test.go` 展示了 `SEED=false` 时旧商品的数据回填；`internal/service/order_postgres_test.go` 验证真实 PostgreSQL 并发下单。SQLite 单测不能证明 PostgreSQL 行锁行为。

## 错误、鉴权和响应

所有 Handler 通过 `internal/response` 返回 `{ code, message, data, error, requestId }`，`code=0` 表示成功。业务失败用 `internal/apierr` 中的 `Validation`、`NotFound`、`Conflict` 等；SQL/Provider 原因只记录日志，不暴露给客户端。新列表接口复用 `internal/httpapi/helpers.go` 的限额分页。`X-Request-ID` 会回传并进入 Service Context 以供业务日志关联。

`internal/auth` 负责 HS256 JWT；`middleware.Authenticate` 校验 Bearer 并附带用户身份，`RequireRole("admin")` 限制运营路由。但订单和购物车查询仍须在 SQL 中限定本人。退出登录只清除 refresh cookie，不能即时撤销已签发的无状态 access token。

## 测试与检查

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
docker build -t vue-h5-template-business-service:local .
```

大部分 Service/HTTP 规则测试使用临时 SQLite 文件。PostgreSQL 并发测试在没有 `TEST_DATABASE_URL` 时跳过；CI 启动 PostgreSQL 17 并传该变量执行 `go test -race -coverprofile=coverage.out ./...`。本地应使用一次性测试库/Schema，**不要使用生产库**。`internal/repository/inventory_postgres_test.go` 与 `internal/service/order_postgres_test.go` 会创建并清理自己的测试 Schema。

Go 服务本身没有 TypeScript 类型检查；TS 检查只针对 VitePress 配置：

```bash
cd docs
pnpm install --frozen-lockfile
pnpm docs:typecheck
pnpm docs:build
pnpm docs:dev
```

文档镜像在 `/`、`/zh/`、`/ja/`，改页面时一起更新对应语言和侧栏链接。VitePress 使用内建本地搜索与代码高亮，没有额外 Mermaid 渲染器；架构变化优先用清晰的表格或文字，不依赖只显示为代码块的图。
