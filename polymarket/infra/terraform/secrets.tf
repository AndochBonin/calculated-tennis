locals {
  secrets_manager_secret_name = coalesce(var.secrets_manager_secret_name, "${var.project_name}/credentials")
}

resource "aws_secretsmanager_secret" "app" {
  name                    = local.secrets_manager_secret_name
  description             = "Polymarket credentials JSON (set value out of band; see docs/aws-secrets-manager.md)."
  recovery_window_in_days = var.secret_recovery_window_in_days
  kms_key_id              = var.secret_kms_key_id
}

# Optional placeholder version so the secret is readable before a real put-secret-value.
# Real credentials should be applied with: aws secretsmanager put-secret-value --secret-string file://secret.json
resource "aws_secretsmanager_secret_version" "placeholder" {
  count     = var.secret_create_placeholder_version ? 1 : 0
  secret_id = aws_secretsmanager_secret.app.id

  secret_string = jsonencode({
    _placeholder = "Replace with aws secretsmanager put-secret-value --secret-string file://secret.json"
  })

  lifecycle {
    ignore_changes = [secret_string]
  }
}
