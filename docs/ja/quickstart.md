# クイックスタート

Docker Compose（推奨）またはローカルの PostgreSQL に対して Go バイナリを直接実行する方法で、1 分以内にビジネスサービスを起動します。

## 前提条件

- **Docker** と **Docker Compose** — コンテナ化された導入手順用。または
- **Go 1.25+** と **PostgreSQL 16+** — ベアメタルでの導入手順用。

## 方法 A — Docker Compose

サービスを PostgreSQL 17 インスタンスと一緒に起動し、マイグレーションとデモデータのシードを実行します。

```bash
cp .env.example .env
docker compose up --build
```

API は `http://localhost:8002` で待ち受けます。確認してみましょう：

```bash
curl http://localhost:8002/health
# {"code":0,"message":"ok","data":{"service":"business","status":"ok","env":"development","time":"..."},"error":null,"requestId":"..."}

curl http://localhost:8002/api/product/list
```

## 方法 B — ソースから実行

```bash
cp .env.example .env
# DATABASE_URL をローカルの PostgreSQL に向けてから：
make run
```

`make run` は `go run ./cmd/server` を実行します。設定は環境変数から読み込まれます。`.env` の各値は
[設定リファレンス](/ja/configuration) に記載されています。

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

対話型の非ストリーミングエンドポイントは [API リファレンス](/ja/api) に記載されています。

## 開発ループ

```bash
make check     # format + vet + test
make test-race # race detector + coverage
make lint      # golangci-lint
```

## 次のステップ

- [設定リファレンス](/ja/configuration) — すべての環境変数。
- [アーキテクチャ](/ja/architecture) — レイヤーの組み立て方。
- [デプロイ](/ja/deployment) — 本番環境へのリリース。
