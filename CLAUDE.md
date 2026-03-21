# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A distributed SaaS payment processing platform using event-driven microservices on AWS (EKS, SQS, DynamoDB). Designed for ephemeral, cost-conscious infrastructure (~$5/month target).

## Architecture

**Saga Orchestration Pattern**: The payment-service orchestrates multi-step payment flows across services using SQS queues and DynamoDB outbox tables. Each service implements the Transactional Outbox Pattern for reliable event publishing.

**Service topology** (all communicate via SQS, no direct HTTP calls between services):
- **payment-service** (Go/Gorilla Mux, :8081) — Saga orchestrator for payment flows
- **wallet-service** (Go/Gorilla Mux, :8083) — Manages user wallet balances and reservations
- **ledger-service** (Go/Gorilla Mux, :8082) — Event-driven audit trail and transaction log
- **gateway-service** (Node.js/Express, :8084) — Mock external payment gateway
- **notification-service** (Node.js/Express+SSE, :8085) — Real-time notifications via SSE
- **platform-dashboard** (Go stdlib + static HTML, :3000) — Admin dashboard

All services expose `/healthz` for health checks. Internal port is 8080 in containers, mapped to above ports via docker-compose.

## Build & Run Commands

### Local development (full stack)
```bash
docker-compose up --build          # Start all services + LocalStack
./scripts/init-localstack.sh       # Create SQS queues & DynamoDB tables in LocalStack
```

### Go services (payment, wallet, ledger, platform-dashboard)
```bash
# Go workspace covers all Go services
go build ./...                     # Build all Go services
go test ./...                      # Test all Go services
cd apps/<service> && go build -o server ./cmd/server  # Build single service
cd apps/<service> && go test ./...                     # Test single service
```

### Node.js services (gateway, notification)
```bash
cd apps/<service> && npm install && npm start
```

### Infrastructure (Terraform)
```bash
cd envs/dev && terraform init && terraform plan    # Preview changes
cd envs/dev && terraform apply                     # Apply changes
```

### Deployment
```bash
# Dashboard (only service with a deploy script)
./scripts/deploy_dashboard.ps1     # PowerShell: build, tag, push to ECR

# Kubernetes
./scripts/connect-kubectl.sh       # Configure kubectl for EKS
./scripts/debug-k8s.sh             # K8s debugging utilities
```

## Infrastructure Layout

- `envs/dev/` — Dev environment Terraform root (VPC, EKS, SQS/DynamoDB, NGINX Ingress)
- `envs/state-bootstrap/` — S3 backend + DynamoDB lock table for Terraform state
- `modules/network/` — VPC with 2 AZs, public/private subnets (10.0.0.0/16)
- `modules/compute/` — EKS v1.29, managed node groups, IAM roles
- `modules/messaging/` — SQS queues (5 + DLQs), DynamoDB tables (9), outbox pattern with GSIs

## Key Patterns

- **Outbox tables** have a `published-index` GSI for polling unpublished events
- **Idempotency**: Dedicated DynamoDB table prevents duplicate payment processing
- **DLQs**: Every SQS queue has a dead-letter queue (maxReceiveCount: 3)
- **K8s deployments**: 2 replicas each, resource limits set, liveness/readiness probes on `/healthz`
- Go services share deps: AWS SDK v2, gorilla/mux, google/uuid
- Node services share deps: AWS SDK v3, express, uuid
