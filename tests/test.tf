provider "aws" {
  region = "us-east-1"
}

//provider "better-secrets" {}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = "bconnito-test-"
}
