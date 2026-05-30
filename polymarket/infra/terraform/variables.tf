variable "aws_region" {
  description = "AWS region for EC2, S3, and Secrets Manager."
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Name prefix for tagged resources (bucket, secret, IAM, security group)."
  type        = string
  default     = "polymarket"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "project_name must contain only lowercase letters, digits, and hyphens."
  }
}

variable "artifact_prefix" {
  description = "S3 key prefix for artifact objects (trailing slash; used in IAM ListBucket conditions)."
  type        = string
  default     = "artifacts/"

  validation {
    condition     = can(regex("/$", var.artifact_prefix))
    error_message = "artifact_prefix must end with a trailing slash."
  }
}

variable "artifact_key" {
  description = "S3 object key for the Linux amd64 binary (used by make push-binary and EC2 user-data)."
  type        = string
  default     = "artifacts/polymarket"
}

variable "s3_versioning_enabled" {
  description = "Enable S3 versioning on the artifact bucket for rollback."
  type        = bool
  default     = true
}

variable "secrets_manager_secret_name" {
  description = "Secrets Manager secret name (POLYMARKET_SECRETS_MANAGER_SECRET_ID on EC2). Defaults to <project_name>/credentials."
  type        = string
  default     = null
}

variable "secret_recovery_window_in_days" {
  description = "Days to retain the secret after deletion (0 = immediate delete, requires force_destroy)."
  type        = number
  default     = 7

  validation {
    condition     = var.secret_recovery_window_in_days == 0 || (var.secret_recovery_window_in_days >= 7 && var.secret_recovery_window_in_days <= 30)
    error_message = "secret_recovery_window_in_days must be 0 or between 7 and 30."
  }
}

variable "secret_create_placeholder_version" {
  description = "If true, create a placeholder secret version in Terraform (ignored after apply). Prefer false and set JSON via put-secret-value."
  type        = bool
  default     = false
}

variable "secret_kms_key_id" {
  description = "Optional customer-managed KMS key ARN or ID for the secret. When set, EC2 role receives kms:Decrypt on this key."
  type        = string
  default     = null
}

variable "ec2_instance_type" {
  description = "EC2 instance type for the application host."
  type        = string
  default     = "t3.micro"
}

variable "ec2_associate_public_ip" {
  description = "Associate a public IP in the default VPC (useful for SSH before SSM)."
  type        = bool
  default     = true
}

variable "ec2_key_name" {
  description = "Optional EC2 key pair name for SSH access."
  type        = string
  default     = null
}

variable "ssh_allowed_cidr" {
  description = "CIDR allowed to SSH (port 22) to the instance. When null, SSH ingress is disabled."
  type        = string
  default     = null
}

variable "ec2_root_volume_size_gb" {
  description = "Root EBS volume size in GiB."
  type        = number
  default     = 20

  validation {
    condition     = var.ec2_root_volume_size_gb >= 8
    error_message = "ec2_root_volume_size_gb must be at least 8."
  }
}
