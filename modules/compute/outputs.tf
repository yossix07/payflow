# =============================================================================
# COMPUTE MODULE - OUTPUTS
# =============================================================================
# These outputs are CRITICAL for the next phase (Helm Bootstrap).
# The Helm provider needs to know how to connect to the cluster we just built.
# =============================================================================

output "cluster_name" {
  description = "The name of the EKS cluster"
  value       = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  description = "The URL of the EKS API Server (used by kubectl/helm)"
  value       = aws_eks_cluster.main.endpoint
}

output "cluster_certificate_authority_data" {
  description = "Authentication certificate for the cluster"
  value       = aws_eks_cluster.main.certificate_authority[0].data
}
