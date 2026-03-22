# PayFlow

Distributed payment processing platform using event-driven microservices on AWS.

## Architecture

```
                    ┌─────────────────┐
                    │   Dashboard     │ :3000
                    │  (Go + HTML)    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │ Payment Service │ :8081  ← Saga Orchestrator
                    │     (Go)        │
                    └────────┬────────┘
                             │ SQS
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
     ┌────────────┐  ┌──────────────┐  ┌────────────────┐
     │   Wallet   │  │   Gateway    │  │  Notification  │
     │  Service   │  │   Service    │  │    Service     │
     │   (Go)     │  │  (Node.js)   │  │   (Node.js)   │
     │   :8083    │  │    :8084     │  │    :8085       │
     └────────────┘  └──────────────┘  └────────────────┘
              │              │              │ SSE
              ▼              ▼              ▼
         DynamoDB        DynamoDB      Browser
```

**Pattern:** Saga orchestration with transactional outbox. Services communicate via SQS queues — no direct HTTP calls between services.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go (Gorilla Mux), Node.js (Express) |
| Messaging | AWS SQS (with DLQs) |
| Database | AWS DynamoDB (GSIs, conditional writes, TTL) |
| Infrastructure | Terraform (modular), Kubernetes (EKS v1.29), Helm |
| Networking | AWS VPC (multi-AZ), NGINX Ingress |
| Real-time | Server-Sent Events (SSE) |
| Local Dev | Docker Compose, LocalStack |

## Key Patterns

- **Optimistic locking** — DynamoDB version-based conditional writes prevent lost updates
- **Idempotency** — Deduplication tables with conditional puts prevent duplicate processing
- **Transactional outbox** — Reliable event publishing with adaptive polling and poison pill handling
- **Saga timeout & compensation** — Automatic detection and recovery of stuck payments
- **Graceful shutdown** — Connection draining on SIGTERM via WaitGroup (Go) / shutdown flag (Node.js)
- **Structured logging** — JSON output with contextual fields across all services

## Quick Start (Local)

```bash
# Prerequisites: Docker, Node.js

# Start all services + LocalStack
npm run env:up

# Open dashboard
npm run demo

# Trigger a payment flow
npm run demo:trigger

# Check service status
npm run env:status

# Stop everything
npm run env:down
```

## Testing

```bash
npm run test:unit          # Unit tests (all Go services)
npm run test:integration   # Integration tests (requires env:up)
npm test                   # Both
```

## Deploy to AWS

```bash
# Prerequisites: AWS credentials in .aws.env, Terraform, kubectl

# Bootstrap Terraform state (one-time)
cd envs/state-bootstrap && terraform init && terraform apply

# Deploy dev environment
cd envs/dev && terraform init && terraform apply

# Connect kubectl
./scripts/connect-kubectl.sh

# Tear down
cd envs/dev && terraform destroy
```

## Project Structure

```
apps/
  payment-service/     Go — Saga orchestrator, timeout scanner
  wallet-service/      Go — Balance management, fund reservations
  ledger-service/      Go — Immutable audit trail
  gateway-service/     Node.js — Mock payment gateway
  notification-service/ Node.js — Real-time SSE notifications
  platform-dashboard/  Go — Admin dashboard (static HTML)
modules/
  network/             Terraform — VPC, subnets, NAT
  compute/             Terraform — EKS, node groups, IAM
  messaging/           Terraform — SQS queues, DynamoDB tables
envs/
  dev/                 Dev environment (~$5/month)
  prod/                Production config (not deployed)
k8s/
  prod/                HPA, PDB manifests
```
