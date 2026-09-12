resource "azurerm_resource_group" "example" {
  name     = var.resource_group_name
  location = var.location
}

# A system topic sourced at the resource group. Locally delivers a
# Microsoft.Resources.ResourceWriteSuccess / ResourceDeleteSuccess event to it
# whenever a resource inside that scope is created or deleted.
resource "azurerm_eventgrid_system_topic" "example" {
  name                = var.system_topic_name
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  source_resource_id  = azurerm_resource_group.example.id
  topic_type          = "Microsoft.Resources.ResourceGroups"
}

resource "azurerm_eventgrid_system_topic_event_subscription" "example" {
  name                = var.subscription_name
  system_topic        = azurerm_eventgrid_system_topic.example.name
  resource_group_name = azurerm_resource_group.example.name

  event_delivery_schema = "EventGridSchema"

  webhook_endpoint {
    url = var.consumer_webhook_url
  }
}

# Trigger: created after the subscription exists, so its creation produces a
# ResourceWriteSuccess event delivered to the consumer. In real use you'd just
# create resources in this resource group with any tool and watch them arrive.
resource "azurerm_servicebus_namespace" "trigger" {
  name                = var.trigger_namespace_name
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  sku                 = "Standard"

  depends_on = [azurerm_eventgrid_system_topic_event_subscription.example]
}
