# =============================================================================
# COMPUTE MODULE - EKS & NODES
# =============================================================================
# This module creates the "Brain" (Control Plane) and the "Body" (Worker Nodes)
# of our Kubernetes cluster.
#
# COMPONENTS:
#   1. IAM Roles: Permissions for EKS to manage AWS resources
#   2. EKS Cluster: The Master Nodes (Managed by AWS)
#   3. Node Group: The Worker Nodes (EC2 Instances)
# =============================================================================


# =============================================================================
# 1. IAM ROLE FOR EKS CONTROL PLANE
# =============================================================================
# The EKS service itself needs permissions to create Load Balancers, 
# manage ENIs (Network Interfaces), etc. on your behalf.
# =============================================================================

resource "aws_iam_role" "eks_cluster" {
  name = "${var.env}-eks-cluster-role"

  # Trust Policy: "I trust the 'eks.amazonaws.com' service to assume this role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${var.env}-eks-cluster-role"
  }
}

# Attach the standard EKS Cluster Policy
resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster.name
}

# =============================================================================
# ELB PERMISSIONS FOR LOAD BALANCER SERVICE CONTROLLER
# =============================================================================
# The Kubernetes service-controller needs these permissions to create and 
# manage Elastic Load Balancers when services of type LoadBalancer are created.
# The standard AmazonEKSClusterPolicy does NOT include these permissions.
# =============================================================================
resource "aws_iam_role_policy" "eks_elb_permissions" {
  name = "${var.env}-eks-elb-policy"
  role = aws_iam_role.eks_cluster.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "elasticloadbalancing:*",
          "ec2:DescribeAccountAttributes",
          "ec2:DescribeAddresses",
          "ec2:DescribeInternetGateways"
        ]
        Resource = "*"
      }
    ]
  })
}


# =============================================================================
# 2. EKS CLUSTER (Control Plane)
# =============================================================================
resource "aws_eks_cluster" "main" {
  name     = "${var.env}-cluster"
  role_arn = aws_iam_role.eks_cluster.arn

  # EKS Version: Stick to N-1 (stable) or latest if starting fresh.
  version = "1.29" 

  vpc_config {
    # The subnets where EKS places its network interfaces (ENIs)
    # We pass in the PRIVATE subnets
    subnet_ids = var.subnet_ids
    
    # Enable private access so nodes can talk to master silently
    endpoint_private_access = true
    # Enable public access so YOU (the developer) can run kubectl from home
    endpoint_public_access  = true
  }

  # Ensure permissions are created before creating the cluster
  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy
  ]

  tags = {
    Name = "${var.env}-cluster"
  }
}


# =============================================================================
# 3. IAM ROLE FOR WORKER NODES
# =============================================================================
# The actual EC2 instances (Nodes) need permissions to reach ECR (Docker Registry),
# CNI (Networking), and CloudWatch (Logs).
# =============================================================================

resource "aws_iam_role" "eks_nodes" {
  name = "${var.env}-eks-node-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
}

# Attach standard Worker Node Policies
resource "aws_iam_role_policy_attachment" "eks_worker_node_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "eks_cni_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "eks_container_registry_readonly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_nodes.name
}


# =============================================================================
# 4. MANAGED NODE GROUP (Worker Nodes)
# =============================================================================
# This creates the Auto Scaling Group of EC2 instances.
#
# COST STRATEGY: SPOT INSTANCES
# We use capacity_type = "SPOT" to save huge money.
# Trade-off: AWS can reclaim these instances with 2 minutes warning.
# For EKS, this is fine - Kubernetes just moves the pods to another node!
# =============================================================================

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.env}-node-group"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  
  # Put nodes in Private Subnets
  subnet_ids      = var.subnet_ids

  # SCALING CONFIG
  # Start small. Warning: t3.medium needs ~2 nodes for system overhead + app
  scaling_config {
    desired_size = 2
    max_size     = 3
    min_size     = 1
  }

  # INSTANCE TYPES
  # t3.medium = 2 vCPU, 4GB RAM. Good balance.
  # t3.small is too small for EKS (system pods take all RAM).
  instance_types = ["t3.medium"]

  # COST STRATEGY: ON-DEMAND (Fallback from Spot)
  # We switched to ON_DEMAND because Spot capacity was unavailable/failing.
  # Cost: ~$0.04/hour per node (instead of $0.004).
  # For short-term learning, this is acceptable.
  capacity_type = "ON_DEMAND"

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.eks_container_registry_readonly,
  ]

  tags = {
    Name = "${var.env}-node"
    "k8s.io/cluster-autoscaler/enabled" = "true"
  }
}
