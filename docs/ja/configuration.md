# 設定

起動時に作業ディレクトリの任意の `.env` を読み込み、その後プロセス環境変数から設定を取得します。既存の変数は明示的な空値も含めて優先されます。ファイルがなければ環境変数と既定値を使い、読込や構文に問題があれば起動を中止します。`.env.development` / `.env.production` ローダーはありません。`.env.example` をプロジェクト直下の `.env` にコピーし、`DATABASE_URL` を設定してください。`.env` は Git の対象外です。秘密情報をコミットしないでください。Compose の `.env` は変数展開（ここでは主に `POSTGRES_PORT`）に使われ、API コンテナの環境は `docker-compose.yml` に明示されています。ネイティブ開発ではプロジェクト設定をシェルで export する必要はありません。`TEST_DATABASE_URL` は PostgreSQL 統合テスト専用で、`config.Load()` の設定ではありません。

設定は**フェイルファスト**です。無効または安全でない値は起動を中止します。

`.env.example` にはネイティブ開発に必要な接続設定と開発用シークレットのみを記載しています。省略した設定には以下のデフォルト値が適用されます。

## 全リファレンス

| 変数 | デフォルト | 説明 |
|---|---|---|
| `APP_ENV` | `development` | `development`、`test`、`production` のいずれか。 |
| `PORT` | `8002` | HTTP 待ち受けポート。 |
| `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` のいずれか。 |
| `LOG_FORMAT` | 本番以外 `text`、本番 `json` | 本番では `json` が必須。 |
| `DATABASE_DRIVER` | `postgres` | 実行時は PostgreSQL。SQLite は `APP_ENV=test` のみ。 |
| `DATABASE_URL` | `postgres://…` | ドライバ固有の DSN。 |
| `DB_MAX_OPEN_CONNS` | `25` | データベースへの最大同時接続数。 |
| `DB_MAX_IDLE_CONNS` | `5` | プールに保持する最大アイドル接続数。 |
| `DB_CONN_MAX_LIFETIME` | `30m` | 接続を再利用できる時間。 |
| `AUTO_MIGRATE` | staging/production `false`; otherwise `true` | 起動時にスキーママイグレーションを実行。 |
| `SEED` | 本番以外 `true`、本番 `false` | 開発用デモデータ。本番は `true` を拒否。 |
| `JWT_SECRET` | dev placeholder | AI サービスと**必ず**一致させる。32 文字以上。 |
| `JWT_ISSUER` | `vue-h5-template` | `iss` クレーム。AI サービスと一致させる。 |
| `JWT_AUDIENCE` | `vue-h5-template-api` | `aud` クレーム。AI サービスと一致させる。 |
| `ACCESS_TOKEN_TTL` | `2h` | アクセストークンの有効期間。 |
| `REFRESH_TOKEN_TTL` | `336h` | リフレッシュトークンの有効期間（`ACCESS_TOKEN_TTL` より長く）。 |
| `REFRESH_COOKIE_NAME` | `vh5_refresh` | リフレッシュトークンを運ぶ HttpOnly Cookie。名前が Go net/http の検証に失敗すると起動しません。 |
| `REFRESH_COOKIE_SECURE` | 本番以外 `false`、本番 `true` | リフレッシュ Cookie に `Secure` を付与。 |
| `CORS_ORIGINS` | `http://localhost:5173,…` | カンマ区切りのブラウザオリジン許可リスト。 |
| `RATE_LIMIT_ENABLED` | `true` | IP 単位のトークンバケットを有効化。 |
| `RATE_LIMIT_RPS` | `20` | IP ごとの持続リクエスト数/秒。 |
| `RATE_LIMIT_BURST` | `40` | IP ごとのバケット容量。 |
| `ORDER_PAYMENT_TTL` | `15m` | 支払い待ち注文の確保期限。 |
| `ORDER_EXPIRATION_INTERVAL` | `30s` | 期限切れ Worker の間隔。 |
| `ORDER_EXPIRATION_BATCH_SIZE` | `100` | 一回の処理で試行する注文数（最大 1000）。注文ごとに別トランザクション。 |
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

## 設定の解析とリソース制限

既定値は未設定の変数にのみ適用されます。整数、真偽値、期間を明示的に空にした場合や形式が不正な場合は起動を拒否します。空の `DATABASE_URL` も拒否します。ポートは 1–65535（テストのみ `0` を許可）、DB のアイドル接続数は最大接続数以下、HTTP と終了のタイムアウトは正の値が必要です。レート制限は NaN と無限大を拒否し、ログレベルと形式は上記の値に限ります。

## 隔離テスト用 SQLite

SQLite は `APP_ENV=test` の場合のみ許可し、開発・本番の実行には PostgreSQL を使用します。パッケージ内の SQLite テストには CGO が必要です。行ロックと並行注文のテストには専用 PostgreSQL を指す `TEST_DATABASE_URL` が必要で、各テストは独立したスキーマを作成・削除します。

移行と更新セッションの公開手順は[本番動作](/ja/production)を参照。`local` に対応し、`staging` は本番安全検証を適用します。`AUTO_MIGRATE` は staging/production で false、他では true。`DB_CONN_MAX_IDLE_TIME` は 5m、`REQUEST_TIMEOUT` は 10s で `WRITE_TIMEOUT` より短くします。
