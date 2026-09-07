# クイックスタート

Go ビジネス API を起動します。このリポジトリには H5 画面はなく、`docs/` は独立した VitePress プロジェクトです。

## 前提条件

- 完全なローカル構成には **Docker Compose**、ソース実行には **Go 1.25+** と PostgreSQL が必要です。
- ドキュメントの編集・ビルドに限り **Node.js 22+** と **pnpm 11** を使用します。

## 方法 A — Docker Compose

既存の Compose は API、PostgreSQL 17、Redis 8 を起動します。開発時にはマイグレーションとデモアカウント・商品・SKU・クーポンのシードを実行します。Compose は環境変数を直接指定しており、`.env.example` のコピーだけでは JWT や DB 接続先は変わりません。

```bash
docker compose up -d --build
```

API は `http://localhost:8002` で待ち受けます。確認してみましょう：

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
docker compose ps
```

## 方法 B — ソースから実行

```bash
# PostgreSQL だけを起動し、既存の 5432 番ポートを避けます。
POSTGRES_PORT=5433 docker compose up -d postgres
export DATABASE_URL='postgres://vue_h5:vue_h5_local@127.0.0.1:5433/vue_h5_business?sslmode=disable'
go run ./cmd/server
```

`make run` も同じ Go コマンドを実行します。Compose の `business` サービスはホストの 8002 番ポートを使うため、同時に起動しないでください。Go プロセスは **`.env` を自動読込しません**。シェルで export するか env-file ツールを使ってください。`.env.example` には 5432 番ポート用の開発認証情報があり、上記の手順では 5433 に変更します。Redis は任意で、ネイティブ実行時の `REDIS_URL` 既定値は空です。標準の Go モジュールプロキシがタイムアウトする場合は `GOPROXY=https://goproxy.cn go mod download` を先に実行し、リポジトリの `go.sum` 検証は有効のままにしてください。詳細は[設定](/ja/configuration)。

`DATABASE_URL` を設定せずに `go run ./cmd/server` を実行すると、既定の `localhost:5432` に接続します。そこが別アプリのデータベースである可能性があります。本サービスを起動するために既存テーブルを移行・削除せず、上記の専用 Compose データベースと接続 URL を使用してください。起動時はアプリのマイグレーション前に互換性のない `users`/`products` スキーマを拒否します。

## API を試す

シード済みアカウントでログインし、カタログを取得します：

```bash
# 1. 認証（accessToken を返し、リフレッシュ Cookie も設定します）
curl -s http://localhost:8002/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"user","password":"123456"}'

# 2. 手順 1 のトークンでプロフィールを取得
curl -s http://localhost:8002/api/user/info \
  -H 'Authorization: Bearer <accessToken>'

# 3. 商品をお気に入り登録
curl -s http://localhost:8002/api/product/favorite \
  -H 'Authorization: Bearer <accessToken>' \
  -H 'Content-Type: application/json' \
  -d '{"productId":1,"favorite":true}'
```

注文の手順は[取引フロー](/ja/commerce)、詳細な API は [API リファレンス](/ja/api) を参照してください。

## 開発ループ

```bash
make check     # format + vet + test
make test-race # race detector + coverage
make lint      # golangci-lint
make build     # bin/server にコンパイル

cd docs
pnpm install --frozen-lockfile
pnpm docs:dev
pnpm docs:build
```

## 次のステップ

- [設定リファレンス](/ja/configuration) — すべての環境変数。
- [アーキテクチャ](/ja/architecture) — レイヤーの組み立て方。
- [デプロイ](/ja/deployment) — 本番環境へのリリース。
- [サービスの拡張](/ja/development) — API、モデル、テストの追加。
