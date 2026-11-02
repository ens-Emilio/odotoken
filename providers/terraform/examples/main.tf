terraform {
  required_providers {
    odotoken = {
      source  = "odotoken/odotoken"
      version = "~> 0.1"
    }
  }
}

provider "odotoken" {
  endpoint = "http://localhost:8080"
  api_key  = var.odotoken_api_key
}

variable "odotoken_api_key" {
  type      = string
  sensitive = true
}

# Gerenciar chave de API
resource "odotoken_client_key" "app_key" {
  name         = "minha-aplicacao"
  rate_limit   = 1000
  budget_limit = 50.00
}

# Regra de preço customizada
resource "odotoken_price_rule" "gpt4_discount" {
  model_pattern = "gpt-4o"
  client_id     = odotoken_client_key.app_key.id
  input_price   = 2.50
  output_price  = 10.00
  priority      = 10
}

# Política de segurança
resource "odotoken_guard_policy" "default" {
  name                 = "politica-padrao"
  enabled              = true
  max_tokens           = 4096
  allow_patterns       = ["gpt-4o", "claude-3.5-sonnet"]
  block_patterns       = ["[INST]", "ignore previous"]
  max_requests_per_min = 60
}

# Data sources
data "odotoken_health" "status" {}

data "odotoken_models" "available" {}

output "health_status" {
  value = data.odotoken_health.status.status
}

output "available_models" {
  value = data.odotoken_models.available.models
}

output "client_key_id" {
  value = odotoken_client_key.app_key.id
}