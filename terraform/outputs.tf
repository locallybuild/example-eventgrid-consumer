output "topic_endpoint" {
  description = "Endpoint to publish events to."
  value       = azurerm_eventgrid_topic.example.endpoint
}

output "topic_primary_access_key" {
  description = "Access key for publishing (aeg-sas-key header)."
  value       = azurerm_eventgrid_topic.example.primary_access_key
  sensitive   = true
}
