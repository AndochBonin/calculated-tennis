data "aws_iam_policy_document" "ec2_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "ec2" {
  name               = "${var.project_name}-ec2"
  assume_role_policy = data.aws_iam_policy_document.ec2_assume_role.json
}

data "aws_iam_policy_document" "ec2_secrets_read" {
  statement {
    sid    = "SecretsManagerGetSecretValue"
    effect = "Allow"

    actions = [
      "secretsmanager:GetSecretValue",
    ]

    resources = [aws_secretsmanager_secret.app.arn]
  }

  dynamic "statement" {
    for_each = var.secret_kms_key_id != null ? [1] : []
    content {
      sid    = "KMSDecryptSecret"
      effect = "Allow"

      actions = [
        "kms:Decrypt",
      ]

      resources = [var.secret_kms_key_id]
    }
  }
}

resource "aws_iam_role_policy" "ec2_secrets_read" {
  name   = "${var.project_name}-ec2-secrets-read"
  role   = aws_iam_role.ec2.id
  policy = data.aws_iam_policy_document.ec2_secrets_read.json
}

data "aws_iam_policy_document" "ec2_s3_artifacts" {
  statement {
    sid    = "S3GetArtifactObject"
    effect = "Allow"

    actions = [
      "s3:GetObject",
    ]

    resources = [
      "${aws_s3_bucket.artifacts.arn}/${var.artifact_key}",
    ]
  }

  statement {
    sid    = "S3ListArtifactPrefix"
    effect = "Allow"

    actions = [
      "s3:ListBucket",
    ]

    resources = [
      aws_s3_bucket.artifacts.arn,
    ]

    condition {
      test     = "StringLike"
      variable = "s3:prefix"

      values = [
        "${var.artifact_prefix}*",
      ]
    }
  }
}

resource "aws_iam_role_policy" "ec2_s3_artifacts" {
  name   = "${var.project_name}-ec2-s3-artifacts"
  role   = aws_iam_role.ec2.id
  policy = data.aws_iam_policy_document.ec2_s3_artifacts.json
}

resource "aws_iam_instance_profile" "ec2" {
  name = "${var.project_name}-ec2"
  role = aws_iam_role.ec2.name
}
