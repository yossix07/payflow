# SaaS-Ready Production Cluster

## Overview

This project implements a cost-efficient, scalable, and secure "SaaS-Ready Production Cluster" using AWS, EKS, and Terraform. It leverages ephemeral infrastructure concepts (Spin-up / Tear-down) to minimize costs during development and testing.

## Technology Stack

- **Infrastructure as Code:** Terraform
- **Cloud Provider:** AWS
- **Orchestration:** Kubernetes (EKS)
- **Package Management:** Helm

## Directory Structure

- `modules/`: Reusable Terraform modules (Network, Compute, Bootstrap).
- `envs/`: Environment-specific configurations (Dev, Prod).

## Prerequisites

- AWS CLI configured
- Terraform installed
- kubectl installed

## FinOps Strategy

Designed with a strict budget of $5.00/month for learning purposes. Ensure you have an AWS Budget set up before deploying.
