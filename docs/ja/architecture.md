# アーキテクチャ

これは単一の Go プロセスです。Gin が HTTP、Service が業務ルール、GORM Repository が PostgreSQL を担当します。このリポジトリにフロントエンド `src/`、画面ルーター、コンポーネント、クライアント Store、BFF、WebSocket、SSE はありません。Redis は商品カタログだけをキャッシュし、AI ストリーミングは別リポジトリです。

```
cmd/server          エントリポイント：配線、ライフサイクル、グレースフルシャットダウン
internal/config     フェイルファストな環境設定
internal/model      永続化エンティティ（一部に旧 JSON フィールドタグを保持）
internal/database   接続、バージョン管理されたマイグレーション、冪等なシード
internal/repository データアクセス（GORM）
internal/service    ビジネスルール — 判断を下す唯一のレイヤー
internal/httpapi    Gin ルーター、ハンドラー、ミドルウェア
internal/response   共有 JSON エンベロープ
internal/apierr     安定したアプリケーションエラーコード
internal/auth       JWT の発行 / 検証
internal/logging    構造化 JSON / テキストロギング
internal/cache      任意の Redis 商品カタログキャッシュ
internal/metrics    独立した Prometheus レジストリ
docs/               独立した VitePress サイト（pnpm）
```

実際の経路は `H5 クライアント → Gin ミドルウェア/ハンドラー → Service → Repository → PostgreSQL` です。商品詳細は Redis のカタログキャッシュを使えても、在庫は毎回 PostgreSQL から取得します。`cmd/server/main.go` は HTTP と期限切れ Worker を起動し、シャットダウンを管理します。

## リクエストライフサイクル

1. **ミドルウェア**がリクエスト ID、セキュリティヘッダー、パニック回復、アクセスログ、Prometheus、CORS、ボディサイズ制限を扱います。有効時、`/api` にはインスタンス内 IP レート制限も適用します。
2. **ハンドラー**は入力のバインドのみを行い、サービスメソッドを呼び出します。
3. **サービス**がビジネスルールを強制し、失敗時は `*apierr.Error` を返します。
4. **ハンドラー**が `response.OK` / `response.Fail` でエンベロープを書き出します。
5. **未知のルート**も同じ JSON エンベロープを返します。保護ルートは JWT を検証し、管理ルートは `admin` ロールを要求します。

## エラーモデル

すべての失敗は、互いに独立した三つの情報を持つ `apierr.Error` です：

- **HTTP ステータス**（クライアント、ロードバランサー、可観測性のため）；
- 安定した**アプリケーションコード**（`4xxx` / `5xxx`）— クライアントコードで分岐しても安全；
- **人間が読めるメッセージ** — エンドユーザーに表示しても安全。

未知のエラーは不透明な `5000` に正規化されます — 元の原因はログに記録されますが、クライアントに漏れることはありません。

## 認証フロー

- **ログイン**は認証情報を検証し（定数時間の bcrypt、失敗パスではダミーハッシュでタイミングを平準化）、
  アクセストークンとリフレッシュトークンを発行します。リフレッシュトークンは `HttpOnly` クッキーに書き込まれ、
  アクセストークンはボディで返されます。
- **アクセストークン**は短命（`2h`）で、`Authorization: Bearer …` として送信されます。
- **リフレッシュ**は Cookie（または非ブラウザクライアントのボディ）から新しいトークン対を発行します。サーバー側セッション/リフレッシュトークン保存やアクセスの即時失効はありません。ログアウトは Cookie を消すため、クライアント側でアクセストークンも破棄してください。
- 他サービスでこの JWT を受け入れる場合は、`JWT_SECRET`、`JWT_ISSUER`、`JWT_AUDIENCE` をそこで一致させてください。このリポジトリは別の AI サービスを実装・検証しません。

## データ

- PostgreSQL が実行時のデータの基準です。SQLite は隔離テスト・デモ専用で、PostgreSQL の行ロックの証明には使えません。
- `internal/database/migrate.go` は `AUTO_MIGRATE=true` で順序付きマイグレーションを実行し、既存商品のデフォルト SKU を補完します。`internal/database/seed.go` は `SEED=true` のときだけデモアカウント・商品・クーポンを投入し、既存の表を上書きしません。
- SKU、注文、決済金額は `int64` の最小通貨単位です。Product の文字列価格は旧 H5 互換用で、注文金額には使いません。

## 取引の整合性

- 注文、在庫確保、商品スナップショット、クーポン確保、カート消去は同一トランザクションです。
- `available >= quantity` の条件付き更新が超過販売を防ぎ、`(user_id, idempotency_key)` の一意制約が重複注文を防ぎます。
- `(provider, event_id)` は Webhook を重複排除します。キャンセルと期限切れは状態変更と同時に在庫・クーポンを戻します。
- 複数インスタンスの Worker は `FOR UPDATE SKIP LOCKED` と注文バージョン検証を使います。
- Redis に注文や在庫は保存せず、商品変更時にカタログキャッシュを無効化します。

詳細は[取引フロー](/ja/commerce)、拡張手順は[サービスの拡張](/ja/development)を参照してください。
