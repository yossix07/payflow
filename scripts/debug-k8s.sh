#!/bin/sh
set -e

echo "Installing kubectl..."
curl -sLO "https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl"
chmod +x kubectl
mv kubectl /usr/local/bin/

echo "Configuring kubeconfig..."
aws eks update-kubeconfig --region us-east-1 --name dev-cluster

echo ""
echo "=========================================="
echo "1. CLUSTER INFO"
echo "=========================================="
kubectl cluster-info

echo ""
echo "=========================================="
echo "2. NODE STATUS"
echo "=========================================="
kubectl get nodes -o wide

echo ""
echo "=========================================="
echo "3. ALL PODS STATUS"
echo "=========================================="
kubectl get pods -A

echo ""
echo "=========================================="
echo "4. INGRESS-NGINX NAMESPACE DETAILS"
echo "=========================================="
kubectl get all -n ingress-nginx

echo ""
echo "=========================================="
echo "5. INGRESS CONTROLLER PODS (Detailed)"
echo "=========================================="
kubectl describe pods -n ingress-nginx -l app.kubernetes.io/component=controller

echo ""
echo "=========================================="
echo "6. INGRESS SERVICE DETAILS"
echo "=========================================="
kubectl describe svc -n ingress-nginx

echo ""
echo "=========================================="
echo "7. RECENT EVENTS IN INGRESS NAMESPACE"
echo "=========================================="
kubectl get events -n ingress-nginx --sort-by='.lastTimestamp' | tail -20

echo ""
echo "=========================================="
echo "8. LOAD BALANCER STATUS"
echo "=========================================="
kubectl get svc -n ingress-nginx -o wide

echo ""
echo "=========================================="
echo "9. CHECKING FOR SUBNET TAGS (via AWS CLI)"
echo "=========================================="
aws ec2 describe-subnets --filters "Name=tag:Name,Values=*dev*" --query 'Subnets[*].[SubnetId,CidrBlock,Tags[?Key==`kubernetes.io/role/elb`].Value | [0],Tags[?Key==`Name`].Value | [0]]' --output table

echo ""
echo "Debug complete!"
