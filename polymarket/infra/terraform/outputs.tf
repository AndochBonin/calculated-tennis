output "aws_region" {
  description = "Region where resources are deployed."
  value       = var.aws_region
}

output "project_name" {
  description = "Resource name prefix."
  value       = var.project_name
}

output "artifact_bucket" {
  description = "S3 bucket for application binaries (make push-binary, EC2 user-data)."
  value       = aws_s3_bucket.artifacts.id
}

output "artifact_key_prefix" {
  description = "S3 key prefix for artifacts (IAM ListBucket prefix conditions)."
  value       = var.artifact_prefix
}

output "artifact_key" {
  description = "S3 object key for the application binary."
  value       = var.artifact_key
}

output "default_vpc_id" {
  description = "Default VPC used for the EC2 instance."
  value       = local.default_vpc_id
}

output "default_subnet_ids" {
  description = "Default subnets (one per AZ) in the default VPC."
  value       = data.aws_subnets.default.ids
}

output "secrets_manager_secret_name" {
  description = "Secrets Manager secret name (set POLYMARKET_SECRETS_MANAGER_SECRET_ID to this on EC2)."
  value       = aws_secretsmanager_secret.app.name
}

output "secrets_manager_secret_arn" {
  description = "Secrets Manager secret ARN."
  value       = aws_secretsmanager_secret.app.arn
}

output "ec2_instance_id" {
  description = "Application EC2 instance ID."
  value       = aws_instance.app.id
}

output "ec2_public_ip" {
  description = "Public IPv4 of the application instance (null if no public IP)."
  value       = aws_instance.app.public_ip
}

output "ec2_private_ip" {
  description = "Private IPv4 of the application instance."
  value       = aws_instance.app.private_ip
}

output "ec2_security_group_id" {
  description = "Security group attached to the application instance."
  value       = aws_security_group.app.id
}

output "ec2_instance_profile_name" {
  description = "IAM instance profile for the EC2 instance (Secrets Manager + S3 artifact read)."
  value       = aws_iam_instance_profile.ec2.name
}

output "ec2_instance_profile_arn" {
  description = "IAM instance profile ARN."
  value       = aws_iam_instance_profile.ec2.arn
}

output "ec2_iam_role_name" {
  description = "IAM role attached to the EC2 instance profile."
  value       = aws_iam_role.ec2.name
}
