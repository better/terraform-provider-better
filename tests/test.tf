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
  password = "fake_fake_fake"

  # DB
  db_engine           = "postgres"
  db_admin_username   = "test_admin"
  db_service_username = "test_service"
  db_ro_username      = "test_ro"

  # MQ
  mq_engine_type        = "ActiveMQ"
  mq_engine_version     = "5.15.14"
  mq_host_instance_type = "mq.t3.micro"
  mq_broker_name        = "tfp-test-mq"
  mq_username           = "test"

  # Cache
  cache_auth_token                    = "dummy_password_99999999999"
  cache_cluster_name                  = "tfp-test-redis"
  cache_engine_version                = "5.0.6"
  cache_node_type                     = "cache.t2.micro"
  cache_replication_group_description = "${local.cache_cluster_name} Redis Cache Cluster"
  cache_subnet_group_name             = "ops-01-elasticache"
  cache_port                          = "6379"
  cache_security_group_ids            = ["sg-0df26abf9ad205709"]
}

# Database
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
  engine              = local.db_engine
  allocated_storage   = 5
  publicly_accessible = false
  skip_final_snapshot = true

  username = local.db_admin_username
  password = local.password

  apply_immediately = true

  lifecycle {
    ignore_changes = [password]
  }
}

resource "sdm_resource" "db_admin" {
  postgres {
    name = "${local.prefix}${local.db_admin_username}"

    hostname = aws_db_instance.db.address
    port     = aws_db_instance.db.port

    username = local.db_admin_username
    password = local.password

    database = local.db_engine
  }
}

resource "sdm_resource" "db_service" {
  postgres {
    name = "${local.prefix}${local.db_service_username}"

    hostname = aws_db_instance.db.address
    port     = aws_db_instance.db.port

    username = local.db_service_username
    password = local.password

    database = local.db_engine
  }
}

resource "sdm_resource" "db_ro" {
  postgres {
    name = "${local.prefix}${local.db_ro_username}"

    hostname = aws_db_instance.db.address
    port     = aws_db_instance.db.port

    username = local.db_ro_username
    password = local.password

    database = local.db_engine
  }
}

resource "better_database_password_association" "better_admin" {
  secret_id = better_database_password.db.secret_id
  db_id     = aws_db_instance.db.id
  db_users = [
    {
      key    = "ADMIN_PASSWORD",
      sdm_id = sdm_resource.db_admin.id,
    },
    {
      key    = "USER_PASSWORD",
      sdm_id = sdm_resource.db_service.id
    },
    {
      key    = "READONLY_USER_PASSWORD",
      sdm_id = sdm_resource.db_ro.id
    }
  ]
}

# MQ
resource "aws_secretsmanager_secret" "mq" {
  name_prefix             = local.prefix
  recovery_window_in_days = 0
}

resource "aws_mq_broker" "mq" {

  broker_name = local.mq_broker_name

  engine_type    = local.mq_engine_type
  engine_version = local.mq_engine_version

  security_groups = ["sg-1d60117b"]

  host_instance_type = "mq.t3.micro"

  user {
    username       = "admin"
    password       = local.password
    console_access = true
  }

  user {
    username       = local.mq_username
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
      user           = local.mq_username
      key            = "USER_PASSWORD"
      console_access = false
    }
  ]
}

# ElastiCache
resource "aws_secretsmanager_secret" "cache" {
  name_prefix             = local.prefix
  recovery_window_in_days = 0
}

resource "aws_elasticache_replication_group" "cache" {
  engine                        = "redis"
  transit_encryption_enabled    = true
  engine_version                = local.cache_engine_version
  node_type                     = local.cache_node_type
  number_cache_clusters         = 1
  replication_group_description = local.cache_replication_group_description
  replication_group_id          = local.cache_cluster_name
  auth_token                    = local.cache_auth_token
  subnet_group_name             = local.cache_subnet_group_name
  security_group_ids            = local.cache_security_group_ids
}

resource "sdm_resource" "cache" {
  elasticache_redis {
    name         = local.cache_cluster_name
    hostname     = aws_elasticache_replication_group.cache.reader_endpoint_address
    password     = local.cache_auth_token
    port         = local.cache_port
    tls_required = true
  }
}

resource "better_cache_password" "cache" {
  secret_id = aws_secretsmanager_secret.cache.id
}

resource "better_cache_password_association" "cache" {
  secret_id            = better_cache_password.cache.secret_id
  replication_group_id = aws_elasticache_replication_group.cache.id
  sdm_id               = sdm_resource.cache.id
}
