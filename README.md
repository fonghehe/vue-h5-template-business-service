# vue-h5-template-business-service

The **business API** for [vue-h5-template](https://github.com/fonghehe/vue-h5-template) — authentication,
user profile and favourites, and the product catalogue. Built with Go, Gin and GORM, backed by PostgreSQL.

It is one half of the backend pair. Streaming AI workloads live in the sibling
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service); the two services share a JWT
secret and a response envelope so the frontend needs only one client.

## Highlights

- **Frontend-aligned contract** — every response is `{ code, message, data, error, requestId }` with `code === 0`
  meaning success, matching `@vh5/api-client`.
- **Layered architecture** — `config → model → repository → service → httpapi`, with a versioned migration and a
  deterministic seed.
- **Production hygiene** — fail-fast config validation, rate limiting, CORS and trusted-proxy hardening, request
  correlation ids, structured JSON logs, graceful shutdown.
- **Ops-ready** — multi-stage non-root Docker image, health/readiness probes, `docker compose` with PostgreSQL,
  GitHub Actions CI (fmt + vet + race + coverage + lint + image build).

## Quick start

```bash
cp .env.example .env
docker compose up --build          # or: make run (needs local PostgreSQL)
```

The API listens on `http://localhost:8002`. Try the public catalogue:

```bash
curl http://localhost:8002/api/product/list
```

## Documentation

The documentation is written with VitePress and published to GitHub Pages at
<https://fonghehe.github.io/vue-h5-template-business-service/>. It is available in three languages —
**English** (primary), **简体中文** and **日本語**:

| Language | URL |
|---|---|
| English | <https://fonghehe.github.io/vue-h5-template-business-service/> |
| 简体中文 | <https://fonghehe.github.io/vue-h5-template-business-service/zh/> |
| 日本語 | <https://fonghehe.github.io/vue-h5-template-business-service/ja/> |

Preview locally:

```bash
cd docs && npm install && npm run docs:dev
```

Publishing is automated: `.github/workflows/deploy-docs.yml` builds and deploys the site whenever `docs/`
changes on `main`.

## Development

```bash
make check     # fmt + vet + test
make test-race # race detector + coverage
make lint      # golangci-lint
```

## License

[MIT](./LICENSE)
