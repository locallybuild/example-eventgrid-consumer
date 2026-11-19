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

  # Terraform is run via `locally run terraform ...`, which injects
  # ARM_* / AZURE_* credentials and endpoints. Locally does not implement
  # resource-provider registration, so skip it.
  resource_provider_registrations = "none"
}
