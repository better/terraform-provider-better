provider "aws" {
  region = "us-east-1"
}

provider "secrets" {}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = "tfp-secrets-test-"
}
