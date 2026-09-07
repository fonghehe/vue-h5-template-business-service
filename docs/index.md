---
layout: home

hero:
  name: "business-service"
  text: "Commerce business API for vue-h5-template"
  tagline: Auth, SKU inventory, cart, coupons and transactional orders in one Go service.
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
  - title: Transactional checkout
    details: Server-priced SKUs, conditional inventory reservation, coupon limits, order snapshots and cart clearing commit together.
  - title: Retry-safe payment state
    details: Database-backed order and webhook idempotency, explicit state transitions, cancellation and expiration release.
  - title: Existing H5 contract
    details: Auth, profile, favourites and product endpoints remain available with the shared JSON envelope.
  - title: Operable service
    details: PostgreSQL source of truth, optional Redis catalogue cache, structured logs, Prometheus metrics and multi-instance-safe expiration worker.
---

## The backend pair

This is a Go HTTP service, not the H5 frontend: there are no pages, Vue components, client stores or Vite app here. Streaming AI workloads live in the sibling
[`vue-h5-template-ai-service`](https://github.com/fonghehe/vue-h5-template-ai-service). The two services share
a JWT contract and response envelope; the frontend routes requests to each service separately.

| | Business service (Go) | AI service (Python) |
|---|---|---|
| Workload | Transactional commerce and account data | Long-lived streaming |
| Scaling | Request rate | Concurrent streams |
| Failure mode | Database latency | Upstream model latency |

## Demo accounts

Development seeding runs when `SEED=true`; production rejects it. Never deploy these credentials:

| Username | Password | Roles |
|---|---|---|
| `user` | `123456` | user |
| `admin` | `123456` | user, admin |

Start with [Quick start](/quickstart), follow one checkout in [Commerce flow](/commerce), then use [Extend the service](/development) for a new endpoint or model.
