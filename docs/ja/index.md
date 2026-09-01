---
layout: home

hero:
  name: "business-service"
  text: "vue-h5-template のビジネス API"
  tagline: 認証、ユーザープロフィールとお気に入り、商品カタログ — Go・Gin・GORM を PostgreSQL 上で構成。
  actions:
    - theme: brand
      text: クイックスタート
      link: /ja/quickstart
    - theme: alt
      text: API リファレンス
      link: /ja/api
    - theme: alt
      text: GitHub で見る
      link: https://github.com/fonghehe/vue-h5-template-business-service

features:
  - title: フロントエンドに整合した契約
    details: すべてのレスポンスは <code>{ code, message, data, error, requestId }</code> で、<code>code === 0</code> が成功を意味します。<code>@vh5/api-client</code> と完全に一致します。
  - title: レイヤード・アーキテクチャ
    details: config → model → repository → service → httpapi。バージョン管理されたマイグレーションと、決定的で冪等なシードを備えています。
  - title: 本番運用のための設計
    details: フェイルファストな設定検証、IP 単位のレート制限、CORS と信頼済みプロキシの強化、リクエスト相関 ID、構造化 JSON ログ。
  - title: 運用即応
    details: マルチステージの非 root Docker イメージ、ヘルス/レディネスプローブ、PostgreSQL 付き docker compose、GitHub Actions CI。
---

## バックエンドの二本柱

これは vue-h5-template バックエンドの片割れです。ストリーミング AI のワークロードは兄弟リポジトリ
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service) にあります。二つのサービスは
JWT シークレットとレスポンスエンベロープを共有するため、フロントエンドはクライアントを一つ用意するだけで済みます。

| | ビジネスサービス (Go) | AI サービス (Python) |
|---|---|---|
| ワークロード | 短いトランザクション CRUD | 長時間のストリーミング |
| スケーリング | リクエストレート | 同時ストリーム数 |
| 障害モード | データベース遅延 | 上流モデルの遅延 |

## デモアカウント

空のデータベースではシードが自動実行されます：

| ユーザー名 | パスワード | ロール |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |
