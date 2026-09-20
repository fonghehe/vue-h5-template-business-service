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
# 初回のみコピーし、プロジェクト内で DB 接続先を設定します。
cp .env.example .env
# 任意：ローカル DB がない場合は Compose で PostgreSQL を起動します。
POSTGRES_PORT=5433 docker compose up -d --wait postgres
go run ./cmd/server
```

`make run` も同じ Go コマンドを実行します。Compose の `business` サービスはホストの 8002 番ポートを使うため、同時に起動しないでください。サーバーは作業ディレクトリの `.env` を自動で読み込むため、リポジトリ直下から実行してください。既存のプロセス環境変数が優先されます。`.env.example` はポート 5433 を使います。ローカル PostgreSQL がある場合は `.env` の `DATABASE_URL` を専用 DB に設定し、Compose の起動を省略できます。Redis は任意で、ネイティブ実行時の `REDIS_URL` 既定値は空です。標準の Go モジュールプロキシがタイムアウトする場合は `GOPROXY=https://goproxy.cn go mod download` を先に実行し、リポジトリの `go.sum` 検証は有効のままにしてください。詳細は[設定](/ja/configuration)。

`.env` も明示的な `DATABASE_URL` もない状態で `go run ./cmd/server` を実行すると、既定の `localhost:5432` に接続します。そこが別アプリのデータベースである可能性があります。本サービスを起動するために既存テーブルを移行・削除せず、専用データベースを使い、接続 URL をプロジェクトの `.env` に設定してください。起動時はアプリのマイグレーション前に互換性のない `users`/`products` スキーマを拒否します。

## 対話型 API ドキュメント

開発環境の Swagger UI は `http://localhost:8002/swagger/index.html` から開けます。現在の OpenAPI 契約は `/openapi.yaml` です。ページは `github.com/swaggo/gin-swagger` で登録し、リソースは `github.com/swaggo/files` が提供します。別のドキュメントサーバーや CDN は不要です。既存の OpenAPI 3 契約を読み込むため、`swag init` で Swagger 2 文書を生成する必要はありません。`APP_ENV=production` ではこれらのルートを登録しません。

`POST /api/auth/login` で開発アカウント `user` / `123456` を使ってログインし、`data.accessToken` を **Authorize** に入力してください。管理 API は `admin` / `123456` を使います。決済はモックで、実際の課金は発生しません。

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
