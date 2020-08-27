provider "aws" {
  region = "us-east-1"
}

provider "secrets" {}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = "tfp-secrets-test-"
  recovery_window_in_days = 0
}

resource "secrets_database_password" "this" {
  secret_id = aws_secretsmanager_secret.this.id
}
