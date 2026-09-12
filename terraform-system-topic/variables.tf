variable "location" {
  type    = string
  default = "berlin"
}

variable "resource_group_name" {
  type    = string
  default = "example-eventgrid-systopic"
}

variable "system_topic_name" {
  type    = string
  default = "example-rg-events"
}

variable "subscription_name" {
  type    = string
  default = "example-resource-events"
}

variable "consumer_webhook_url" {
  type        = string
  description = "URL the system-topic subscription delivers resource events to (https required)."
  default     = "https://localhost:8080/events"
}

variable "trigger_namespace_name" {
  type        = string
  description = "A sample resource created inside the watched resource group to fire an event."
  default     = "example-trigger-ns"
}
