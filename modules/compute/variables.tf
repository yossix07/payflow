# =============================================================================
# COMPUTE MODULE - VARIABLES
# =============================================================================

variable "env" {
  description = "The environment name (e.g., dev, prod)"
  type        = string
}

variable "vpc_id" {
  description = "The ID of the VPC where the cluster will be created"
  type        = string
}

variable "subnet_ids" {
  description = "List of PRIVATE subnet IDs for the worker nodes"
  type        = list(string)
  
  # WHY PRIVATE SUBNETS?
  # Security! Worker nodes should never be directly accessible from the internet.
  # They should run your apps in a private network, and traffic should come 
  # in via a Load Balancer (in public subnets).
}
