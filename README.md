# SaaS-Ready Production Cluster

## Overview

This project implements a cost-efficient, scalable, and secure "SaaS-Ready Production Cluster" using AWS, EKS, and Terraform. It leverages ephemeral infrastructure concepts (spin-up / tear-down) to minimize costs during development and testing.

## Technology Stack

- **Infrastructure as Code:** Terraform
- **Cloud Provider:** AWS
- **Orchestration:** Kubernetes (EKS)
- **Package Management:** Helm

## Directory Structure

- `modules/`: Reusable Terraform modules (Network, Compute, Bootstrap).
- `envs/`: Environment-specific configurations (Dev, Prod).

---

## 🚀 Deployment Guide

This guide details how to deploy the infrastructure from scratch using Docker to ensure reproducibility.

### 1. Prerequisites

**Tools:**

- **Docker**: Installed and running (we use Docker to run Terraform/AWS CLI).
- **Git**: To clone the repository.

**AWS Permissions:**
You need an AWS Account with **AdministratorAccess** or the following more-specific permissions:

- **EC2**: Create instances, VPCs, Security Groups, Load Balancers.
- **EKS**: Create clusters and node groups.
- **IAM**: Create roles and policies.
- **S3**: For Terraform state storage (if using remote state).
- **VPC**: Manage networking components.

**Credentials:**
Create a file named `.aws.env` in the root of the project:

```env
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
# AWS_SESSION_TOKEN=your_token (Required if using temporary credentials)
AWS_REGION=us-east-1
ECR_REGISTRY_URL=your_account_id.dkr.ecr.us-east-1.amazonaws.com
```

### 2. Deploy Infrastructure

Run the following commands from the project root.

**Step A: Build & Push Applications**
_(Required for initial setup or after code changes)_

```powershell
powershell -ExecutionPolicy Bypass -File scripts/deploy_dashboard.ps1
```

**Step B: Initialize Terraform**

```powershell
cd envs/dev
docker run --rm -v "${PWD}/../../:/workspace" -w /workspace/envs/dev --env-file ../../.aws.env hashicorp/terraform:light init
```

**Step B: Plan Deployment**

```powershell
docker run --rm -v "${PWD}/../../:/workspace" -w /workspace/envs/dev --env-file ../../.aws.env hashicorp/terraform:light plan
```

**Step C: Apply Changes**

```powershell
docker run --rm -v "${PWD}/../../:/workspace" -w /workspace/envs/dev --env-file ../../.aws.env hashicorp/terraform:light apply -auto-approve
```

---

## 🔍 Verification & Access

After deployment, the AWS Load Balancer takes a few minutes to provision. Use this command to fetch your public Entry Point URL.

**Get Load Balancer URL:**

```powershell
docker run --rm --env-file ../../.aws.env --entrypoint sh amazon/aws-cli -c "aws sts get-caller-identity && curl -s -LO https://dl.k8s.io/release/v1.29.0/bin/linux/amd64/kubectl && chmod +x kubectl && aws eks update-kubeconfig --name dev-cluster --region us-east-1 > /dev/null && kubectl get svc nginx-ingress-ingress-nginx-controller -n ingress-nginx -o jsonpath='http://{.status.loadBalancer.ingress[0].hostname}'"
```

> **Note:** Accessing this URL will likely return a `404 Not Found` (Nginx default page). This confirms the infrastructure is working and routing traffic correctly to the cluster!

---

## 🧹 Tear Down (Destroy)

To destroy all resources and stop incurring costs:

```powershell
docker run --rm -v "${PWD}/../../:/workspace" -w /workspace/envs/dev --env-file ../../.aws.env hashicorp/terraform:light destroy -auto-approve
```

## FinOps Strategy

Designed with a strict budget of $5.00/month for learning purposes. Ensure you have an AWS Budget set up before deploying.
