---
layout: home

hero:
  name: "business-service"
  text: "vue-h5-template のコマース API"
  tagline: 認証、SKU 在庫、カート、クーポン、トランザクション注文を一つの Go サービスで提供します。
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
  - title: トランザクション注文
    details: サーバー側 SKU 価格、条件付き在庫確保、クーポン、注文スナップショット、カート消去を一括コミットします。
  - title: 再試行に強い決済状態
    details: DB 制約による注文・Webhook の冪等性、状態遷移、キャンセルと期限切れ時の確保解除。
  - title: 既存 H5 契約を維持
    details: 認証、プロフィール、お気に入り、商品 API は共通 JSON エンベロープで継続します。
  - title: 運用可能なサービス
    details: PostgreSQL、任意の Redis 商品キャッシュ、構造化ログ、Prometheus 指標、複数インスタンス対応 Worker。
---

## バックエンドの二本柱

これは Go HTTP サービスで、H5 フロントエンドではありません。ページ、Vue コンポーネント、クライアント Store、Vite アプリはありません。ストリーミング AI は兄弟リポジトリ
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service) にあります。二つのサービスは
JWT 契約とレスポンスエンベロープを共有し、フロントエンドは各サービスにリクエストを振り分けます。

| | ビジネスサービス (Go) | AI サービス (Python) |
|---|---|---|
| ワークロード | トランザクション取引とアカウント | 長時間のストリーミング |
| スケーリング | リクエストレート | 同時ストリーム数 |
| 障害モード | データベース遅延 | 上流モデルの遅延 |

## デモアカウント

デモアカウントは `SEED=true` の開発環境に限ります。本番環境では有効化できません：

| ユーザー名 | パスワード | ロール |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |

[クイックスタート](/ja/quickstart)で起動し、[取引フロー](/ja/commerce)で注文を確認してから、[サービスの拡張](/ja/development)を参照してください。
