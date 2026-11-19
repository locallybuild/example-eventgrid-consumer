terraform {
  required_version = ">= 1.6"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.0"
    }
  }
}

provider "azurerm" {
  features {}

  # Run via `locally run terraform ...`; Locally injects ARM_* / AZURE_* env.
  resource_provider_registrations = "none"
}
