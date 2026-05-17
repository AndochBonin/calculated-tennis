data "aws_ami" "amazon_linux_2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_security_group" "app" {
  name        = "${var.project_name}-app"
  description = "Polymarket EC2: egress for AWS APIs and Polymarket HTTPS; optional SSH."
  vpc_id      = local.default_vpc_id

  egress {
    description = "HTTPS and other outbound (AWS APIs, Polymarket)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  dynamic "ingress" {
    for_each = var.ssh_allowed_cidr != null ? [1] : []
    content {
      description = "SSH from ssh_allowed_cidr"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = [var.ssh_allowed_cidr]
    }
  }

  tags = {
    Name = "${var.project_name}-app"
  }
}

resource "aws_instance" "app" {
  ami                         = data.aws_ami.amazon_linux_2023.id
  instance_type               = var.ec2_instance_type
  subnet_id                   = one(data.aws_subnets.default.ids)
  vpc_security_group_ids      = [aws_security_group.app.id]
  iam_instance_profile        = aws_iam_instance_profile.ec2.name
  key_name                    = var.ec2_key_name
  associate_public_ip_address = var.ec2_associate_public_ip
  user_data_replace_on_change = true

  user_data = templatefile("${path.module}/user-data.tpl", {
    aws_region                  = var.aws_region
    artifact_bucket             = aws_s3_bucket.artifacts.id
    artifact_key                = var.artifact_key
    secrets_manager_secret_name = aws_secretsmanager_secret.app.name
    project_name                = var.project_name
  })

  root_block_device {
    volume_size = var.ec2_root_volume_size_gb
    volume_type = "gp3"
  }

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 2
  }

  tags = {
    Name = "${var.project_name}-app"
  }
}
