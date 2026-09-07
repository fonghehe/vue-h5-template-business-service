# サービスの拡張

このリポジトリは**バックエンド**です。H5 画面、ルーターメニュー、Vue コンポーネント、Composable、クライアント Store、テーマ、フロントエンドの国際化は別のフロントエンドリポジトリで実装します。ここでは Go HTTP API またはビジネストランザクションを追加します。

## 既存 API をたどる

`GET /api/cart` は実際の認証付き読み取りの例です：

1. `internal/httpapi/router.go`: `cart.GET("", s.getCart)` は `middleware.Authenticate` を使用するグループ内にあります。
2. `internal/httpapi/handlers_commerce.go`: `getCart` は `middleware.CurrentUserID(c)` を取得し、`s.services.Cart.List` を呼び、`response.OK` / `response.Fail` を返します。
3. `internal/service/commerce_service.go`: `CartService.List` は Repository に委譲します。同じファイルの更新処理は数量、販売状態、注文状態のルールを判断します。
4. `internal/repository/commerce.go`: `ListCart` は SQL で `Where("user_id = ?", userID)` を適用し、SKU/Product をプリロードします。所有者の制限は Handler の表示調整ではなく SQL で行います。
5. `internal/httpapi/router_test.go` と `internal/service/commerce_test.go` が HTTP 契約とルールを別々に検証します。

以下は実コードの Handler で、周辺の宣言だけを省略しています：

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

新しい読み取り API は `router.go` にルートを登録し、`internal/httpapi` に小さな入力/応答処理、`internal/service` に永続的な業務ルール、`internal/repository` に SQL を置き、Router/Service テストを追加します。認証・管理者 API は既存の適切なグループに登録します。ユーザー ID、注文価格、状態などの基準情報をクライアント入力から信用しないでください。SQL は文字列結合せずパラメータをバインドします。

## フィールドまたはエンティティの追加

1. `internal/model` に永続化フィールドと JSON 名を追加します。公開済み JSON 名の変更は互換性に影響します。
2. `internal/database/migrate.go` に**新しい**順序付きマイグレーションを追加し、配布済みのものは編集しません。`internal/database/seed.go` は決定的な開発データ専用で、本番マイグレーションは `SEED=true` に依存させません。
3. 不変条件とクエリに合う GORM インデックス/制約を追加します。金額は `int64` の最小通貨単位で、float は使いません。注文アイテムの履歴スナップショットを保ちます。
4. Repository 操作を追加し、複数書き込みには `CommerceRepository.Transaction(ctx, fn)` を使用します。在庫の条件付き更新と `TransitionOrder` を再利用し、Handler で直接状態を代入しません。
5. 公開 API を変更したら、三言語の API 文書とフロントエンドリポジトリの OpenAPI 契約を更新します。

`internal/database/migrate_test.go` は `SEED=false` の旧商品補完、`internal/service/order_postgres_test.go` は PostgreSQL の実並行注文を示します。SQLite の単体テストだけでは PostgreSQL の行ロックを証明できません。

## エラー、認証、応答

全 Handler は `internal/response` から `{ code, message, data, error, requestId }` を返します（成功は `code=0`）。業務エラーには `internal/apierr` の `Validation`、`NotFound`、`Conflict` などを使い、SQL/Provider の内部原因はログに記録して応答に露出しません。新しい一覧 API は `internal/httpapi/helpers.go` の上限付きページングを再利用します。`X-Request-ID` は応答に反映され、Service Context の業務ログにも使用できます。

`internal/auth` が HS256 JWT を扱い、`middleware.Authenticate` が Bearer を検証、`RequireRole("admin")` が管理者 API を制限します。ただし注文・カートの SQL も認証ユーザーに絞る必要があります。ログアウトは refresh cookie を消去しますが、発行済みのステートレス access token を即時失効できません。

## テストとチェック

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make build
docker build -t vue-h5-template-business-service:local .
```

多くの Service/HTTP テストは一時 SQLite ファイルを使います。PostgreSQL 並行テストは `TEST_DATABASE_URL` がなければスキップし、CI は PostgreSQL 17 を起動してこの変数と `go test -race -coverprofile=coverage.out ./...` を実行します。ローカルでは使い捨てテスト DB/Schema を使い、**本番 DB を指定しないでください**。`internal/repository/inventory_postgres_test.go` と `internal/service/order_postgres_test.go` は自身のテスト Schema を作成・削除します。

Go サービスに TypeScript ビルドはありません。TS チェックは VitePress 設定だけです：

```bash
cd docs
pnpm install --frozen-lockfile
pnpm docs:typecheck
pnpm docs:build
pnpm docs:dev
```

文書は `/`、`/zh/`、`/ja/` に対応し、ページと sidebar を一緒に更新します。VitePress の内蔵ローカル検索・コードハイライトを使用し、Mermaid 専用レンダラーはありません。図がコードブロックになることに依存せず、表と文章で構造を記してください。
