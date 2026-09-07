# 設定

Go プロセスは環境変数のみを読み、**`.env` を自動読込しません**。`.env.development` / `.env.production` ローダーもありません。`.env.example` はテンプレートです。Compose の `.env` は変数展開（ここでは主に `POSTGRES_PORT`）に使われ、API コンテナの環境は `docker-compose.yml` に明示されています。ネイティブ起動時はシェルで export してください。`TEST_DATABASE_URL` は PostgreSQL 統合テスト専用で、`config.Load()` の設定ではありません。

設定は**フェイルファスト**です。無効または安全でない値は起動を中止します。

## 全リファレンス

| 変数 | デフォルト | 説明 |
|---|---|---|
| `APP_ENV` | `development` | `development`、`test`、`production` のいずれか。 |
| `PORT` | `8002` | HTTP 待ち受けポート。 |
| `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` のいずれか。 |
| `LOG_FORMAT` | 本番以外 `text`、本番 `json` | 本番では `json` が必須。 |
| `DATABASE_DRIVER` | `postgres` | `postgres` または `sqlite`（sqlite は隔離されたデモ・テスト専用）。 |
| `DATABASE_URL` | `postgres://…` | ドライバ固有の DSN。 |
| `DB_MAX_OPEN_CONNS` | `25` | データベースへの最大同時接続数。 |
| `DB_MAX_IDLE_CONNS` | `5` | プールに保持する最大アイドル接続数。 |
| `DB_CONN_MAX_LIFETIME` | `30m` | 接続を再利用できる時間。 |
| `AUTO_MIGRATE` | `true` | 起動時にスキーママイグレーションを実行。 |
| `SEED` | 本番以外 `true`、本番 `false` | 開発用デモデータ。本番は `true` を拒否。 |
| `JWT_SECRET` | dev placeholder | AI サービスと**必ず**一致させる。32 文字以上。 |
| `JWT_ISSUER` | `vue-h5-template` | `iss` クレーム。AI サービスと一致させる。 |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` クレーム。AI サービスと一致させる。 |
| `ACCESS_TOKEN_TTL` | `2h` | アクセストークンの有効期間。 |
| `REFRESH_TOKEN_TTL` | `336h` | リフレッシュトークンの有効期間（`ACCESS_TOKEN_TTL` より長く）。 |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | リフレッシュトークンを運ぶ HttpOnly Cookie。 |
| `REFRESH_COOKIE_SECURE` | 本番以外 `false`、本番 `true` | リフレッシュ Cookie に `Secure` を付与。 |
| `CORS_ORIGINS` | `http://localhost:5173,…` | カンマ区切りのブラウザオリジン許可リスト。 |
| `RATE_LIMIT_ENABLED` | `true` | IP 単位のトークンバケットを有効化。 |
| `RATE_LIMIT_RPS` | `20` | IP ごとの持続リクエスト数/秒。 |
| `RATE_LIMIT_BURST` | `40` | IP ごとのバケット容量。 |
| `ORDER_PAYMENT_TTL` | `15m` | 支払い待ち注文の確保期限。 |
| `ORDER_EXPIRATION_INTERVAL` | `30s` | 期限切れ Worker の間隔。 |
| `ORDER_EXPIRATION_BATCH_SIZE` | `100` | 1 バッチの上限（最大 1000）。 |
| `MOCK_PAYMENT_WEBHOOK_SECRET` | 開発用プレースホルダー | Mock Webhook の HMAC 鍵。本番では 32 文字以上の別値が必要。 |
| `REDIS_URL` | 空 | 任意の商品カタログ Redis キャッシュ。空なら無効。 |
| `PRODUCT_CACHE_TTL` | `5m` | 商品カタログキャッシュの TTL。 |
| `SHUTDOWN_TIMEOUT` | `15s` | グレースフルシャットダウンの上限。 |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `15s` / `30s` / `60s` | HTTP サーバーのタイムアウト。 |

## 本番環境での検証

本番の既定値は `SEED=false` で、明示的な `true` も起動時に拒否されます。デモアカウントとクーポンを本番に入れません。

`APP_ENV=production` の場合、起動は以下のすべてが満たされない限り実行を拒否します：

- `JWT_SECRET` がデフォルトのプレースホルダーではなく、32 文字以上であること。
- `MOCK_PAYMENT_WEBHOOK_SECRET` がデフォルト値ではなく、32 文字以上であること。
- `REFRESH_COOKIE_SECURE` が `true` であること。
- `CORS_ORIGINS` が明示的なオリジンを列挙していること（`*` やループバックオリジン（`localhost`、`127.0.0.1` など）は不可）。
- `LOG_FORMAT` が `json` であること。

これは意図的なものです：誤設定のサービスが起動してしまうのは、起動時にクラッシュするよりもはるかに危険です。

## 隔離実行のための SQLite

`DATABASE_DRIVER=sqlite` は、完全な PostgreSQL が不要なデモや隔離テスト向けにサポートされています。本番用途では
**想定されておらず**、本番では PostgreSQL を使用してください。

```bash
DATABASE_DRIVER=sqlite DATABASE_URL=/tmp/vue-h5-business.db make run
```
