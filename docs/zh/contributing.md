# 贡献指南

感谢你有意参与贡献。本文档介绍工作流程；行为准则见
[CODE_OF_CONDUCT.md](https://github.com/fonghehe/vue-h5-template-business-service/blob/main/CODE_OF_CONDUCT.md)。

## 环境准备

```bash
git clone https://github.com/fonghehe/vue-h5-template-business-service.git
cd vue-h5-template-business-service
```

需要 Go 1.25+。涉及 PostgreSQL 的测试在 CI 的 service 容器上运行；多数单元测试使用临时 SQLite 数据库文件，
无需外部服务。

## 开发命令

```bash
make check     # gofmt + go vet + go test
make test-race # go test -race -coverprofile=coverage.out ./...
make lint      # golangci-lint run ./...
make build     # 编译二进制到 bin/
```

## 代码风格

新增 Go 接口、持久化模型、事务与测试时，按[扩展服务](/zh/development)的真实代码路径操作；只复制 `.env.example` 不会为 Go 进程导出环境变量，启动步骤见[快速开始](/zh/quickstart)。

- 格式化由 `gofmt` 强制，CI 会对未格式化代码报错。
- 静态分析运行 `go vet` 与 `golangci-lint`（配置见 `.golangci.yml`）。
- 分层架构是刻意设计 —— 业务规则属于 `internal/service`，绝不能放进 handler。

## 新增错误码

应用错误码是**公开契约**：一旦发布，某个数字码不得改作他用。请在 `internal/apierr/errors.go` 中新增常量，
而不是复用旧码。

## Pull Request 检查清单

公共 API 变化要同步 `/`、`/zh/`、`/ja/` 三套文档与前端仓库的 OpenAPI 契约。

1. 为改动新增或更新测试。
2. 本地运行 `make check` 并保持全绿。
3. 保持响应信封契约不变 —— 重命名 JSON 字段属于破坏性变更。
4. 接口或配置变化时同步更新 API 参考与配置文档。

## 发布

打上 tag 后，Docker 构建会通过 `git describe` 获取版本号。
