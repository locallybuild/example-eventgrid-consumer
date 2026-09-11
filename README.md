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

## Why the consumer uses TLS

The azurerm provider requires the webhook URL to be `https`, and Locally verifies the
certificate when it delivers. You don't need to generate one: Locally already ships a
`localhost` cert that its own delivery client trusts, at
`~/.config/locally/localhost.pem` (and `localhost.key`). The `make consumer` target and
the default `consumer_webhook_url` (`https://localhost:8080/events`) are set up for this,
so it works with no extra steps.

## Run it end to end

1. Start the consumer over TLS with Locally's cert (leave it running — it answers the
   validation handshake Locally sends before delivering):
   ```bash
   make consumer
   # equivalently:
   # go run ./cmd/consumer \
   #   -tls-cert ~/.config/locally/localhost.pem \
   #   -tls-key  ~/.config/locally/localhost.key
   ```
2. Create the topic and subscription:
   ```bash
   make tf-apply      # locally run terraform -chdir=terraform apply
   ```
3. Fire a sample event:
   ```bash
   make publish
   ```

The event is delivered a few seconds after publishing (Locally retries on a short
interval), and the consumer prints it.

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
| `-tls-cert` | `CONSUMER_TLS_CERT` | _(none)_ |
| `-tls-key` | `CONSUMER_TLS_KEY` | _(none)_ |

With no `-tls-cert`/`-tls-key` the consumer serves plain HTTP, which is handy for poking
at it with `curl`. Set both to serve HTTPS, which is what EventGrid delivery needs.

Terraform variables live in `terraform/variables.tf` — the region (`location`), the
resource/topic/subscription names, and `consumer_webhook_url` (default
`https://localhost:8080/events`).

## Notes

- Locally's data plane is a single local process, so the webhook is reachable at
  `localhost` — no tunnel or container-host address needed.
- Locally sends the subscription-validation handshake just before it first delivers, not
  at subscription-create time, so applying the Terraform without the consumer running
  still succeeds.
- Locally state does not survive a restart. After restarting Locally, run
  `make tf-apply` again to recreate the topic and subscription.
- Tear down with `make tf-destroy`.

## Tests

```bash
go test ./...
```
