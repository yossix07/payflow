# Main configuration for the Dev environment

# =============================================================================
# MODULE: NETWORK
# =============================================================================
# Calling our reusable network module to create the VPC architecture.
# We pass in the specific "Dev" settings here.
# =============================================================================

module "network" {
  # Source: Where is the module code? (Relative path)
  source = "../../modules/network"

  # Inputs (Argument assignment)
  env      = "dev"
  vpc_cidr = "10.0.0.0/16"
  
  # Multi-AZ Deployment (High Availability)
  # Us-east-1a and 1b are generally reliable zones
  azs      = ["us-east-1a", "us-east-1b"]
  
  # Subnet Allocation:
  # VP: 10.0.0.0/16
  #   - Public A: 10.0.1.0/24
  #   - Public B: 10.0.2.0/24
  #   - Private A: 10.0.10.0/24
  #   - Private B: 10.0.11.0/24
  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24"]
  private_subnets = ["10.0.10.0/24", "10.0.11.0/24"]
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

  env        = "dev"
  
  # Dependency Injection:
  # The compute module needs to know WHERE to live.
  # We reference values dynamically from the network module.
  vpc_id     = module.network.vpc_id
  subnet_ids = module.network.private_subnet_ids
}


# =============================================================================
# HELM PROVIDER CONFIGURATION
# =============================================================================
# To install software (Helm Charts) on the cluster, Terraform needs credentials.
# We fetch these dynamically from the AWS EKS API using "data sources".
# =============================================================================

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
  version    = "4.10.0" # Always pin versions!
  
  namespace        = "ingress-nginx"
  create_namespace = true

  # Wait for the Compute module to finish before trying to install!
  depends_on = [module.compute]
  
  # Optimization: Don't wait too long
  timeout = 1200
}


