# =============================================================================
# NETWORK MODULE - VARIABLES
# =============================================================================
# This file defines the "inputs" for our network module.
# Think of a module like a function in Python/JS: these are the arguments.
# =============================================================================

variable "env" {
  description = "The environment name (e.g., dev, prod, staging)"
  type        = string
}

variable "vpc_cidr" {
  description = "The IP address range for the entire VPC (e.g., 10.0.0.0/16)"
  type        = string
  
  # CIDR (Classless Inter-Domain Routing) Quick Guide:
  # /16 = 65,536 IPs (Standard for VPCs)
  # /24 = 256 IPs (Standard for Subnets)
  # /32 = 1 IP (Single Host)
}

variable "azs" {
  description = "List of Availability Zones to deploy into (for High Availability)"
  type        = list(string)
  # Example: ["us-east-1a", "us-east-1b"]
}

variable "private_subnets" {
  description = "List of CIDR blocks for private subnets (one per AZ)"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "public_subnets" {
  description = "List of CIDR blocks for public subnets (one per AZ)"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24"]
}

variable "cluster_name" {
  description = "Name of the EKS cluster (used for subnet tagging)"
  type        = string
  default     = "dev-cluster"
}
