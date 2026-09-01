---
layout: home

hero:
  name: "business-service"
  text: "The business API for vue-h5-template"
  tagline: Authentication, user profile & favourites, and the product catalogue — built with Go, Gin and GORM on PostgreSQL.
  actions:
    - theme: brand
      text: Quick start
      link: /quickstart
    - theme: alt
      text: API reference
      link: /api
    - theme: alt
      text: View on GitHub
      link: https://github.com/fonghehe/vue-h5-template-business-service

features:
  - title: Frontend-aligned contract
    details: Every response is <code>{ code, message, data, error, requestId }</code> with <code>code === 0</code> meaning success, matching <code>@vh5/api-client</code> exactly.
  - title: Layered architecture
    details: config → model → repository → service → httpapi, with a versioned migration and a deterministic, idempotent seed.
  - title: Production hygiene
    details: Fail-fast config validation, per-IP rate limiting, CORS and trusted-proxy hardening, request correlation ids and structured JSON logs.
  - title: Ops-ready
    details: Multi-stage non-root Docker image, health/readiness probes, docker compose with PostgreSQL, and GitHub Actions CI.
---

## The backend pair

This is one half of the vue-h5-template backend. Streaming AI workloads live in the sibling
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service). The two services share
a JWT secret and a response envelope, so the frontend needs only one client.

| | Business service (Go) | AI service (Python) |
|---|---|---|
| Workload | Short, transactional CRUD | Long-lived streaming |
| Scaling | Request rate | Concurrent streams |
| Failure mode | Database latency | Upstream model latency |

## Demo accounts

Seeding runs automatically on an empty database:

| Username | Password | Roles |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |
