provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project = var.project_name
      Managed = "terraform"
    }
  }
}

data "aws_vpcs" "default" {
  filter {
    name   = "is-default"
    values = ["true"]
  }
}

locals {
  default_vpc_id = one(data.aws_vpcs.default.ids)
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [local.default_vpc_id]
  }

  filter {
    name   = "default-for-az"
    values = ["true"]
  }
}
