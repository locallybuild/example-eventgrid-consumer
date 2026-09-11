# example-eventgrid-consumer

A small Go webhook consumer for Azure EventGrid, plus Terraform that provisions a
topic and a webhook subscription. The consumer logs each event it receives, stamped
with the time it arrived. Everything runs locally against
[Locally](https://locally.build/docs).

## What's here

- `cmd/consumer` — HTTP server. Receives EventGrid deliveries at `POST /events`,
  answers the subscription-validation handshake, and prints each event with its
  arrival time. `GET /healthz` returns `ok`.
- `cmd/publisher` — sends one sample event to the topic so you can watch it arrive.
- `internal/events` — EventGrid-schema event types, batch parsing, validation detection.
- `internal/consumer` — the request handler and the readable log format.
- `terraform/` — the topic (`azurerm_eventgrid_topic`) and webhook subscription
  (`azurerm_eventgrid_event_subscription`).

## Prerequisites

- Go 1.27+.
- A running Locally instance. Start it in your own terminal:
  ```bash
  locally build --skip-browser
  ```
  (The dashboard stays at https://localhost:5678 for when you want it.)
- The EventGrid plugin. Check with:
  ```bash
  locally plugin supports Microsoft.EventGrid/topics
  ```

Terraform runs through `locally run`, which points the azurerm provider at Locally and
injects its credentials. You don't configure any endpoints yourself.

## Run it end to end

1. Start the consumer (leave it running — it answers the validation handshake when the
   subscription is created):
   ```bash
   go run ./cmd/consumer
   ```
2. Create the topic and subscription:
   ```bash
   make tf-apply      # locally run terraform -chdir=terraform apply
   ```
3. Fire a sample event:
   ```bash
   make publish
   ```

The consumer prints a block like this for each event:

```
────────────────────────────────────────────
[2026-09-12T14:03:22.145Z] event received
  id:          6f2e...
  eventType:   Example.Items.ItemReceived
  subject:     /example/items/1
  eventTime:   2026-09-12T14:03:22.101Z
  dataVersion: 1.0
  data:
    {
      "itemId": "1234",
      "quantity": 3,
      "message": "hello from the example publisher"
    }
```

## Configuration

Consumer flags (with env fallbacks):

| Flag | Env | Default |
|---|---|---|
| `-addr` | `CONSUMER_ADDR` | `:8080` |
| `-path` | `CONSUMER_PATH` | `/events` |

Terraform variables live in `terraform/variables.tf` — the region (`location`), the
resource/topic/subscription names, and `consumer_webhook_url` (default
`http://localhost:8080/events`).

## Notes

- Locally's data plane is a single local process, so the webhook is reachable at plain
  `http://localhost:8080` — no tunnel or container-host address needed.
- Locally state does not survive a restart. After restarting Locally, run
  `make tf-apply` again to recreate the topic and subscription.
- Tear down with `make tf-destroy`.

## Tests

```bash
go test ./...
```
