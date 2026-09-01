import { defineConfig } from 'vitepress'

const repo = 'https://github.com/fonghehe/vue-h5-template-business-service'

// Deploy to GitHub Pages under the repository sub-path; local dev keeps the
// root base so `npm run docs:dev` works without a path prefix.
const base = process.env.NODE_ENV === 'development' ? '/' : '/vue-h5-template-business-service/'

const en = {
  label: 'English',
  lang: 'en-US',
  title: 'vue-h5-template-business-service',
  description:
    'The business API for vue-h5-template: authentication, user profile and favourites, and the product catalogue.',
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/quickstart' },
      { text: 'API', link: '/api' },
      { text: 'Configuration', link: '/configuration' },
      { text: 'Deployment', link: '/deployment' },
      { text: 'GitHub', link: repo },
    ],
    sidebar: [
      { text: 'Introduction', link: '/' },
      { text: 'Quick start', link: '/quickstart' },
      { text: 'Configuration', link: '/configuration' },
      { text: 'API reference', link: '/api' },
      { text: 'Architecture', link: '/architecture' },
      { text: 'Deployment', link: '/deployment' },
      { text: 'Contributing', link: '/contributing' },
    ],
    outline: { level: [2, 3] as [number, number] },
    editLink: { pattern: `${repo}/edit/main/docs/:path` },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 fonghehe',
    },
  },
}

const zh = {
  label: '简体中文',
  lang: 'zh-CN',
  title: 'vue-h5-template-business-service',
  description:
    'vue-h5-template 的业务 API：鉴权、用户资料与收藏、以及商品目录。',
  themeConfig: {
    nav: [
      { text: '指南', link: '/zh/quickstart' },
      { text: 'API', link: '/zh/api' },
      { text: '配置', link: '/zh/configuration' },
      { text: '部署', link: '/zh/deployment' },
      { text: 'GitHub', link: repo },
    ],
    sidebar: [
      { text: '简介', link: '/zh/' },
      { text: '快速开始', link: '/zh/quickstart' },
      { text: '配置参考', link: '/zh/configuration' },
      { text: 'API 参考', link: '/zh/api' },
      { text: '架构说明', link: '/zh/architecture' },
      { text: '部署指南', link: '/zh/deployment' },
      { text: '贡献指南', link: '/zh/contributing' },
    ],
    outline: { level: [2, 3] as [number, number], label: '本页目录' },
    editLink: { pattern: `${repo}/edit/main/docs/:path`, text: '在 GitHub 上编辑此页' },
    footer: {
      message: '基于 MIT 许可证发布。',
      copyright: 'Copyright © 2026 fonghehe',
    },
  },
}

const ja = {
  label: '日本語',
  lang: 'ja-JP',
  title: 'vue-h5-template-business-service',
  description:
    'vue-h5-template のビジネス API：認証、ユーザープロフィールとお気に入り、商品カタログ。',
  themeConfig: {
    nav: [
      { text: 'ガイド', link: '/ja/quickstart' },
      { text: 'API', link: '/ja/api' },
      { text: '設定', link: '/ja/configuration' },
      { text: 'デプロイ', link: '/ja/deployment' },
      { text: 'GitHub', link: repo },
    ],
    sidebar: [
      { text: 'はじめに', link: '/ja/' },
      { text: 'クイックスタート', link: '/ja/quickstart' },
      { text: '設定リファレンス', link: '/ja/configuration' },
      { text: 'API リファレンス', link: '/ja/api' },
      { text: 'アーキテクチャ', link: '/ja/architecture' },
      { text: 'デプロイ', link: '/ja/deployment' },
      { text: 'コントリビューション', link: '/ja/contributing' },
    ],
    outline: { level: [2, 3] as [number, number], label: 'このページの目次' },
    editLink: { pattern: `${repo}/edit/main/docs/:path`, text: 'GitHub でこのページを編集' },
    footer: {
      message: 'MIT ライセンスの下で公開されています。',
      copyright: 'Copyright © 2026 fonghehe',
    },
  },
}

export default defineConfig({
  lang: 'en-US',
  base,
  cleanUrls: false,
  locales: {
    root: en,
    zh: zh,
    ja: ja,
  },
})
