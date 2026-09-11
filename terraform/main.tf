resource "azurerm_resource_group" "example" {
  name     = var.resource_group_name
  location = var.location
}

resource "azurerm_eventgrid_topic" "example" {
  name                = var.topic_name
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
}

resource "azurerm_eventgrid_event_subscription" "example" {
  name                  = var.subscription_name
  scope                 = azurerm_eventgrid_topic.example.id
  event_delivery_schema = "EventGridSchema"

  webhook_endpoint {
    url = var.consumer_webhook_url
  }
}
