#!/bin/bash
# =============================================================================
# SCRIPT: CONNECT KUBECTL TO EKS
# =============================================================================
# This helper script updates your local ~/.kube/config file so that
# the `kubectl` command knows how to talk to your new EKS cluster.
#
# USAGE:
#   ./connect-kubectl.sh
# 
# PREREQUISITES:
#   - AWS CLI installed and configured
#   - kubectl installed
# =============================================================================

REGION="us-east-1"
CLUSTER_NAME="dev-cluster"
ALIAS="saas-dev"

echo "Connecting to EKS cluster: $CLUSTER_NAME in $REGION..."

aws eks update-kubeconfig \
  --region $REGION \
  --name $CLUSTER_NAME \
  --alias $ALIAS

echo "Done! You can now use: kubectl get nodes"
