# Main configuration for the Prod environment

# =============================================================================
# MODULE: NETWORK
# =============================================================================
# Calling our reusable network module to create the VPC architecture.
# We pass in the specific "Prod" settings here.
# =============================================================================

module "network" {
  source = "../../modules/network"

  env      = "prod"
  vpc_cidr = "10.1.0.0/16"

  # Multi-AZ Deployment (High Availability)
  azs = ["us-east-1a", "us-east-1b"]

  # Subnet Allocation:
  # VPC: 10.1.0.0/16
  #   - Public A:  10.1.1.0/24
  #   - Public B:  10.1.2.0/24
  #   - Private A: 10.1.10.0/24
  #   - Private B: 10.1.11.0/24
  public_subnets  = ["10.1.1.0/24", "10.1.2.0/24"]
  private_subnets = ["10.1.10.0/24", "10.1.11.0/24"]
}

# =============================================================================
# MODULE: COMPUTE
# =============================================================================
# Instantiating the EKS Cluster and Worker Nodes.
# We connect it to the network we just created by passing the outputs
# from module.network to module.compute.
# =============================================================================

module "compute" {
  source = "../../modules/compute"

  env = "prod"

  # Dependency Injection:
  # The compute module needs to know WHERE to live.
  # We reference values dynamically from the network module.
  vpc_id     = module.network.vpc_id
  subnet_ids = module.network.private_subnet_ids
}

# =============================================================================
# MODULE: MESSAGING
# =============================================================================
# SQS queues (with DLQs) and DynamoDB tables for the event-driven services.
# =============================================================================

module "messaging" {
  source = "../../modules/messaging"

  env = "prod"
}

# =============================================================================
# HELM PROVIDER CONFIGURATION
# =============================================================================
# To install software (Helm Charts) on the cluster, Terraform needs credentials.
# Now that the cluster exists, we can use the aws_eks_cluster_auth data source
# to fetch a token natively without needing the AWS CLI installed.
# =============================================================================

data "aws_eks_cluster" "cluster" {
  name = module.compute.cluster_name
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.compute.cluster_name
}

provider "helm" {
  kubernetes {
    host                   = module.compute.cluster_endpoint
    cluster_ca_certificate = base64decode(module.compute.cluster_certificate_authority_data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

# =============================================================================
# HELM RELEASE: NGINX INGRESS CONTROLLER
# =============================================================================
# This installs the "Front Door" of our cluster.
# It creates a Load Balancer (ELB) in AWS that routes traffic to our pods.
# =============================================================================

resource "helm_release" "nginx_ingress" {
  name       = "nginx-ingress"
  repository = "https://kubernetes.github.io/ingress-nginx"
  chart      = "ingress-nginx"
  version    = "4.10.0"

  namespace        = "ingress-nginx"
  create_namespace = true

  depends_on = [module.compute]

  timeout = 1200

  set {
    name  = "defaultBackend.enabled"
    value = "true"
  }

  set {
    name  = "defaultBackend.image.repository"
    value = aws_ecr_repository.platform_dashboard.repository_url
  }

  set {
    name  = "defaultBackend.image.tag"
    value = "latest"
  }

  set {
    name  = "defaultBackend.image.pullPolicy"
    value = "Always"
  }
}

# =============================================================================
# HELM RELEASE: CLUSTER AUTOSCALER
# =============================================================================
# Automatically adjusts the number of nodes in the cluster based on pod
# scheduling demands. Essential for production cost efficiency and resilience.
# =============================================================================

resource "helm_release" "cluster_autoscaler" {
  name       = "cluster-autoscaler"
  repository = "https://kubernetes.github.io/autoscaler"
  chart      = "cluster-autoscaler"
  version    = "9.35.0"

  namespace        = "kube-system"
  create_namespace = false

  depends_on = [module.compute]

  set {
    name  = "autoDiscovery.clusterName"
    value = module.compute.cluster_name
  }

  set {
    name  = "awsRegion"
    value = "us-east-1"
  }
}

# =============================================================================
# ECR REPOSITORIES
# =============================================================================

resource "aws_ecr_repository" "platform_dashboard" {
  name                 = "platform-dashboard"
  image_tag_mutability = "MUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

output "platform_dashboard_repo_url" {
  value = aws_ecr_repository.platform_dashboard.repository_url
}

# =============================================================================
# KUBERNETES DATA & OUTPUTS
# =============================================================================

provider "kubernetes" {
  host                   = module.compute.cluster_endpoint
  cluster_ca_certificate = base64decode(module.compute.cluster_certificate_authority_data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

data "kubernetes_service" "ingress_nginx" {
  metadata {
    name      = "nginx-ingress-ingress-nginx-controller"
    namespace = "ingress-nginx"
  }
  depends_on = [helm_release.nginx_ingress]
}

output "load_balancer_hostname" {
  value = data.kubernetes_service.ingress_nginx.status.0.load_balancer.0.ingress.0.hostname
}
