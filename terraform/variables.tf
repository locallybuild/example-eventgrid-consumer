variable "location" {
  type        = string
  description = "Locally region (confirm valid names with `locally regions list`)."
  default     = "berlin"
}

variable "resource_group_name" {
  type    = string
  default = "example-eventgrid"
}

variable "topic_name" {
  type    = string
  default = "example-topic"
}

variable "subscription_name" {
  type    = string
  default = "example-consumer"
}

variable "consumer_webhook_url" {
  type        = string
  description = "URL the EventGrid subscription delivers to."
  default     = "http://localhost:8080/events"
}
