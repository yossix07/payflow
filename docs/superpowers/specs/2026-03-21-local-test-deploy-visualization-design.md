# Local Testing, Deploy-on-Demand & Real-Time Visualization

**Date:** 2026-03-21
**Status:** Approved

## Goals

1. Establish a centralized local environment that simulates production for testing and development
2. Add unit and integration test infrastructure across all services
3. Enforce "deploy only after testing locally" workflow
4. Build a real-time visualization dashboard showcasing the event-driven architecture
5. Ensure AWS resources are cleaned up to avoid charges — infrastructure off by default

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Task runner | Root `package.json` (npm scripts) | Works natively on Windows without WSL; Node.js already required |
| Visualization | Enhanced platform-dashboard with D3.js animated topology | Lightweight, no extra infra, great CV demo |
| Event streaming | SSE (Server-Sent Events) | One-directional push only; simpler than WebSocket, auto-reconnect, removes `ws` dependency |
| Test layers | Unit (mocked) + Integration (Docker Compose + LocalStack) | Shows both testing levels on CV; integration tests validate real saga flows |
| AWS cost model | Deploy-on-demand — infra down by default | $0 when not demoing; explicit `infra:up`/`infra:down` commands |
| Demo triggers | CLI script + dashboard buttons | CLI for automation/CI, dashboard for interactive demos |

## Architecture Overview

### Local Environment Stack

```
Docker Compose (saas-network)
├── localstack        — SQS queues + DynamoDB tables (simulates AWS)
├── payment-service   — Saga orchestrator (Go, :8081)
├── wallet-service    — Balance management (Go, :8083)
├── ledger-service    — Audit trail (Go, :8082)
├── gateway-service   — Mock payment gateway (Node.js, :8084)
├── notification-service — SSE event broadcaster (Node.js, :8085)
└── platform-dashboard   — Visualization UI (Go, :3000)
```

All services connect to LocalStack via `http://localstack:4566`. Same Docker images, same env var names, same DynamoDB schemas as production — only the AWS endpoint differs.

### npm Scripts (Root package.json)

**Deliverable:** Create a root `package.json` (no application dependencies — scripts only). If parallel script execution is needed, use `concurrently` as a devDependency.

```
npm run env:up           — docker-compose up --build -d + poll /healthz on all services + init LocalStack
npm run env:down         — docker-compose down -v (clean teardown, no leftover volumes)
npm run env:status       — show running containers + health check results

npm run test:unit        — Go + Node unit tests (no Docker needed)
npm run test:integration — env:up → run saga flow tests → env:down
npm run test             — test:unit + test:integration

npm run demo             — env:up + open dashboard in browser
npm run demo:trigger     — CLI script to fire a payment saga flow

npm run infra:up         — terraform apply (deploy to AWS)
npm run infra:down       — terraform destroy (full cleanup)
npm run deploy           — test → build → push ECR → kubectl apply (fails if tests fail)
```

## Test Infrastructure

### Unit Tests (no Docker required)

**Go services** — Go built-in `testing` package with mock implementations via interfaces:

| Service | Test targets |
|---------|-------------|
| payment-service | Saga state machine transitions, idempotency logic, payment handlers |
| wallet-service | Credit/debit logic, fund reservation/release, balance calculations |
| ledger-service | Append-only entry creation, query handlers |
| platform-dashboard | Status endpoint |

**Node services** — Jest with mocked AWS SDK v3 clients:

| Service | Test targets |
|---------|-------------|
| gateway-service | Payment processor logic (success/failure paths), outbox repository |
| notification-service | SSE connection management, event consumer |

### Integration Tests (require Docker Compose + LocalStack)

Located in `tests/integration/` — Go test files that run against the full stack:

**Test scenarios:**
1. **Happy path:** Create payment → wallet reserves funds → gateway processes → ledger records → notification sent
2. **Failure/compensation path:** Gateway rejects → wallet releases reservation → compensation completes
3. **Idempotency:** Same payment ID submitted twice → only processed once

**How they work:**
- HTTP calls to payment-service to initiate flows
- Poll DynamoDB tables directly (via LocalStack) to assert on final state
- Connect to notification-service SSE stream to verify event broadcasts
- 30-second timeout per test — catches stuck sagas

### Test file structure

```
apps/payment-service/internal/saga/orchestrator_test.go
apps/payment-service/internal/handlers/payment_test.go
apps/wallet-service/internal/handlers/wallet_test.go
apps/wallet-service/internal/repository/dynamodb_test.go
apps/ledger-service/internal/handlers/ledger_test.go
apps/gateway-service/src/__tests__/paymentProcessor.test.js
apps/notification-service/src/__tests__/sseManager.test.js
tests/integration/go.mod                              # new module, added to go.work
tests/integration/saga_flow_test.go
tests/integration/idempotency_test.go
```

**Integration test module:** Create `tests/integration/go.mod` and add `./tests/integration` to the root `go.work` file so `go test ./...` discovers these tests.

## Enhanced Platform Dashboard

### Layout

```
┌─────────────────────────────────────────────────────┐
│  SaaS Payment Platform - Live Event Flow            │
├─────────────────────────────────────────────────────┤
│                                                     │
│   [Payment]──→[Wallet]──→[Gateway]                  │
│       │                      │                      │
│       ▼                      ▼                      │
│   [Ledger]            [Notification]                │
│                                                     │
│   Animated SVG: nodes glow on activity,             │
│   arrows pulse with colored particles               │
│   showing message direction                         │
│                                                     │
├─────────────────────────────────────────────────────┤
│  Event Log (scrolling)     │  Saga Status Panel     │
│  10:23:01 PaymentCreated   │  Payment #abc: STARTED │
│  10:23:02 ReserveFunds     │  Wallet: RESERVED      │
│  10:23:03 ProcessPayment   │  Gateway: PROCESSING   │
│  10:23:04 PaymentCompleted │  Status: ✓ COMPLETED   │
├─────────────────────────────────────────────────────┤
│  [▶ Trigger Payment]  [▶ Trigger Failure Demo]      │
└─────────────────────────────────────────────────────┘
```

### Tech stack

- **D3.js** — animated service topology graph (SVG)
- **Vanilla JS + CSS** — no framework, no build step
- Served from existing platform-dashboard Go service as static files

### Animations

- Service nodes glow/pulse when processing an event
- Animated dots travel along arrows showing message direction
- Color coding: green = success, red = failure/compensation, yellow = in-progress
- Event log auto-scrolls with timestamps
- Saga status panel tracks active payment state

### Event streaming

- **notification-service refactored from WebSocket to SSE**
  - Remove `ws` dependency
  - Add `GET /events` SSE endpoint
  - Clients use native `EventSource` API (auto-reconnect built in)
  - Same event data format, just different transport

### Trigger buttons

- **"Trigger Payment"** — POST to platform-dashboard `/api/trigger` which proxies to payment-service `/payments` with random amount + pre-seeded wallet
- **"Trigger Failure Demo"** — same but with parameters causing gateway rejection (demonstrates compensation saga)

### SSE cross-origin access

The dashboard (port 3000) needs to connect to notification-service SSE stream (port 8085). To avoid CORS issues, the **platform-dashboard Go server will proxy the SSE stream** at `/api/events`, forwarding to `notification-service:8080/events` internally. The browser only talks to port 3000.

### Changes to existing services

| Service | Change |
|---------|--------|
| notification-service | Replace WebSocket with SSE (`GET /events` endpoint), remove `ws` package, add Jest devDependency |
| gateway-service | Add Jest devDependency |
| platform-dashboard | New static HTML/JS/CSS dashboard, add `/api/trigger` proxy endpoint, add `/api/events` SSE proxy |
| CLAUDE.md | Update notification-service description from WebSocket to SSE |
| All other services | No changes |

**Note on HTTP calls:** The "no direct HTTP calls between services" rule in CLAUDE.md applies to the saga data path. The dashboard is an operator/admin tool — its `/api/trigger` and `/api/events` proxy endpoints are intentional exceptions, not part of the event-driven saga flow.

## AWS Deploy-on-Demand & Cost Protection

### Philosophy

AWS infrastructure is off by default. Develop and test locally. AWS only exists when actively demoing.

### What `infra:down` destroys

- EKS cluster + node groups (biggest cost)
- NAT Gateway (hourly + per-GB charges)
- Load balancer (created by NGINX Ingress)

### What persists (near-zero cost)

| Resource | Cost when down |
|----------|---------------|
| S3 state bucket | ~$0.01/month |
| DynamoDB lock table | Free tier |
| ECR images | ~$0.10/month |
| Everything else | $0 (destroyed) |

### Safety measures

- `infra:up` prints estimated cost before applying
- `infra:down` runs `terraform destroy` — no lingering resources
- `deploy` refuses to run if tests fail
- ECR repos have `force_delete = true`

### Local ↔ Production parity

Both environments use identical:
- SQS queue names and configurations
- DynamoDB table schemas and GSIs
- Docker images
- Environment variable names

Only difference: `AWS_ENDPOINT` env var — `http://localstack:4566` locally vs real AWS in production.

## Demo Script (CLI)

`scripts/demo-flow.js` — a Node.js script (consistent with Windows-native approach, no bash dependency) that:

1. Seeds a wallet with balance via wallet-service API
2. Triggers a payment via payment-service API
3. Polls payment status and prints colored output showing saga progression
4. Optionally triggers a failure scenario
5. Prints final state summary (wallet balance, ledger entries, payment status)

## Out of Scope (Future)

- Grafana + Prometheus monitoring stack
- CI/CD pipeline (GitHub Actions)
- Load testing (k6)
- OpenAPI/Swagger specs
- Makefile wrapper for Linux/CI environments
