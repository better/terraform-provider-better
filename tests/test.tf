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

  required_version = "~> 0.14.5"
}

provider "aws" {
  region = "us-east-1"
}

locals {
  prefix   = "tfp-better-test-"
  username = "test"
  password = "fake_fake_fake"

  engine = "postgres"

  engine_type        = "ActiveMQ"
  engine_version     = "5.15.14"
  host_instance_type = "mq.t3.micro"
  broker_name        = "test-mq"
}

# # Database
resource "aws_secretsmanager_secret" "db" {
  name_prefix             = local.prefix
  recovery_window_in_days = 0
}

resource "better_database_password" "db" {
  secret_id = aws_secretsmanager_secret.db.id
}

resource "aws_db_instance" "db" {
  identifier_prefix = local.prefix

  instance_class      = "db.t3.micro"
  engine              = local.engine
  allocated_storage   = 5
  publicly_accessible = false
  skip_final_snapshot = true

  username = local.username
  password = local.password

  apply_immediately = true

  lifecycle {
    ignore_changes = [password]
  }
}

resource "sdm_resource" "db" {
  postgres {
    name = "${local.prefix}${local.username}"

    hostname = aws_db_instance.db.address
    port     = aws_db_instance.db.port

    username = local.username
    password = local.password

    database = local.engine
  }
}

resource "better_database_password_association" "db" {
  secret_id = better_database_password.db.secret_id
  key       = "ADMIN_PASSWORD"
  db_id     = aws_db_instance.db.id
  sdm_id    = sdm_resource.db.id
}

# MQ
resource "aws_secretsmanager_secret" "mq" {
  name_prefix             = local.prefix
  recovery_window_in_days = 0
}

resource "aws_mq_broker" "mq" {

  broker_name = local.broker_name

  engine_type    = local.engine_type
  engine_version = local.engine_version

  security_groups = ["sg-1d60117b"]

  host_instance_type = "mq.t3.micro"

  user {
    username       = "admin"
    password       = local.password
    console_access = true
  }

  user {
    username       = local.username
    password       = local.password
    console_access = false
  }
}

resource "sdm_resource" "mq" {
  http_basic_auth {
    name = "MQ-${local.prefix}Admin Console"

    url = aws_mq_broker.mq.instances.0.console_url

    username = "admin"
    password = local.password

    default_path     = "/admin"
    healthcheck_path = "/"
    subdomain        = "mq-${local.prefix}test"
  }
}

resource "better_mq_password" "mq" {
  secret_id = aws_secretsmanager_secret.mq.id
}

resource "better_mq_password_association" "mq_admin" {
  secret_id = better_mq_password.mq.secret_id

  mq_id  = aws_mq_broker.mq.id
  sdm_id = sdm_resource.mq.id

  mq_users = [
    {
      user           = "admin"
      key            = "ADMIN_PASSWORD"
      console_access = true
    },
    {
      user           = local.username
      key            = "USER_PASSWORD"
      console_access = false
    }
  ]
}
