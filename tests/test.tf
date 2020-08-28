provider "aws" {
  region = "us-east-1"
}

provider "better" {}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = "tfp-secrets-test-"
  recovery_window_in_days = 0
}

resource "better_database_password" "this" {
  secret_id = aws_secretsmanager_secret.this.id
}

resource "aws_db_instance" "this" {
  instance_class = "db.t3.micro"

  engine = "mysql"
  engine_version = "5.7"
  storage_type = "gp2"
  allocated_storage = 10
  name = "test"
  parameter_group_name = "default.mysql5.7"
  publicly_accessible = false
  skip_final_snapshot = true

  username = "test"
  password = "fake_fake_fake"

  apply_immediately = true

  lifecycle {
    ignore_changes = [password]
  }
}

resource "better_database_password_association" "this" {
  secret_id = better_database_password.this.secret_id
  rds_db_id = aws_db_instance.this.id
}
