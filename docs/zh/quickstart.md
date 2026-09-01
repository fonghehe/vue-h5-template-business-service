# 快速开始

一分钟内在本地把业务服务跑起来：既可以用 Docker Compose（推荐），也可以直接跑 Go 二进制连接本地 PostgreSQL。

## 前置条件

- **Docker** 与 **Docker Compose** —— 容器化路径；或
- **Go 1.25+** 与 **PostgreSQL 16+** —— 裸机路径。

## 方式 A —— Docker Compose

一次性启动服务与 PostgreSQL 17 实例，自动执行迁移并写入演示数据。

```bash
cp .env.example .env
docker compose up --build
```

API 监听在 `http://localhost:8002`。验证一下：

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
```

## 方式 B —— 从源码运行

```bash
cp .env.example .env
# 把 DATABASE_URL 指向你的本地 PostgreSQL，然后：
make run
```

`make run` 执行 `go run ./cmd/server`。配置从环境变量读取，`.env` 中的每一项说明见
[配置参考](/zh/configuration)。

## 试用 API

用种子账号登录并读取商品目录：

```bash
# 1. 登录（返回 accessToken，同时写入 refresh cookie）
curl -s http://localhost:8002/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"user","password":"123456"}'

# 2. 用上一步的 token 读取个人资料
curl -s http://localhost:8002/api/user/info \
  -H 'Authorization: Bearer <accessToken>'

# 3. 收藏一个商品
curl -s http://localhost:8002/api/product/favorite \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"productId":1,"favorite":true}'
```

各接口的完整说明见 [API 参考](/zh/api)。

## 开发循环

```bash
make check     # 格式化 + vet + 测试
make test-race # 竞态检测 + 覆盖率
make lint      # golangci-lint
```

## 下一步

- [配置参考](/zh/configuration) —— 全部环境变量。
- [架构说明](/zh/architecture) —— 各分层如何协作。
- [部署指南](/zh/deployment) —— 上线到生产。
