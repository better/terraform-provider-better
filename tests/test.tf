terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }

    sdm = {
      source = "terraform.better.com/strongdm/sdm"
    }

    better = {
      source = "terraform.better.com/better/better"
    }
  }

  required_version = "~> 0.13.1"
}

provider "aws" {
  region = "us-east-1"
}

locals {
  prefix = "tfp-better-test-"
  username = "test"
  password = "fake_fake_fake"
  engine = "postgres"
}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = local.prefix
  recovery_window_in_days = 0
}

resource "better_database_password" "this" {
  secret_id = aws_secretsmanager_secret.this.id
}

resource "aws_db_instance" "this" {
  identifier_prefix = local.prefix

  instance_class = "db.t3.micro"
  engine = local.engine
  allocated_storage = 5
  publicly_accessible = false
  skip_final_snapshot = true

  username = local.username
  password = local.password

  apply_immediately = true

  lifecycle {
    ignore_changes = [password]
  }
}

resource "sdm_resource" "this" {
  postgres {
    name = "${local.prefix}${local.username}"

    hostname = aws_db_instance.this.address
    port = aws_db_instance.this.port

    username = local.username
    password = local.password

    database = local.engine
  }
}

resource "better_database_password_association" "this" {
  secret_id = better_database_password.this.secret_id
  key = "ADMIN_PASSWORD"
  db_id = aws_db_instance.this.id
  sdm_id = sdm_resource.this.id
}
