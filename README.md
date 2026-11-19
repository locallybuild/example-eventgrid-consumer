```
ooooo                                      oooo  oooo              
`888'                                      `888  `888              
 888          .ooooo.   .ooooo.   .oooo.    888   888  oooo    ooo 
 888         d88' `88b d88' `"Y8 `P  )88b   888   888   `88.  .8'
 888         888   888 888        .oP"888   888   888    `88..8'
 888       o 888   888 888   .o8 d8(  888   888   888     `888'
o888ooooood8 `Y8bod8P' `Y8bod8P' `Y888""8o o888o o888o     .8'
                                                       .o..P'
                                                       `Y8P'
```

# example-eventgrid-consumer

A small Go webhook consumer for Azure EventGrid, plus Terraform that provisions the
EventGrid resources. The consumer logs each event it receives, stamped with the time it
arrived. Everything runs locally against [Locally](https://locally.build/docs).

There are two demos, because "consume EventGrid events" can mean two different things:

- **Custom topic** — events your own code publishes to a topic. See Demo 1.
- **System topic** — Azure resource lifecycle events, e.g. "a ServiceBus namespace was
  created". This is the "fire on events happening" case. See Demo 2.

## What's here

- `cmd/consumer` — HTTP server. Receives EventGrid deliveries at `POST /events`,
  answers the subscription-validation handshake, and prints each event with its
  arrival time. `GET /healthz` returns `ok`.
- `cmd/publisher` — sends one sample event to a custom topic (Demo 1).
- `internal/events` — EventGrid-schema event types, batch parsing, validation detection.
- `internal/consumer` — the request handler and the readable log format.
- `terraform/` — a custom topic (`azurerm_eventgrid_topic`) and a webhook subscription.
- `terraform-system-topic/` — a system topic on a resource group and a webhook
  subscription, plus a sample resource whose creation fires an event.

## Prerequisites

- Go 1.27+.
- A running Locally instance. Start it in your own terminal:
  ```bash
  locally build --skip-browser
  ```
- The EventGrid plugin, and for Demo 2 the ServiceBus plugin too:
  ```bash
  locally plugin supports Microsoft.EventGrid/topics
  locally plugin supports Microsoft.ServiceBus/namespaces
  ```

Terraform runs through `locally run`, which points the azurerm provider at Locally and
injects its credentials. You don't configure any endpoints yourself.

## The consumer and TLS

EventGrid (via the azurerm provider) requires the webhook URL to be `https`, and Locally
verifies the certificate when it delivers. You don't need to generate one: Locally ships
a `localhost` cert its own delivery client trusts, at `~/.config/locally/localhost.pem`
(and `localhost.key`). `make consumer` points at it, so the loop works with no extra
setup. Both demos deliver to `https://localhost:8080/events`.

```bash
make consumer   # go run ./cmd/consumer -tls-cert ~/.config/locally/localhost.pem -tls-key ~/.config/locally/localhost.key
```

Start it before applying either demo, and leave it running: it answers the
subscription-validation handshakes Locally sends.

## Demo 1 — Custom topic (events you publish)

A custom topic only carries what you explicitly publish to it.

```bash
make consumer      # terminal 1
make tf-init       # terminal 2: download the providers (first run only)
make tf-apply      # create the topic + subscription
make publish       # send a sample event
```

The consumer prints:

```
────────────────────────────────────────────
[2026-09-12T14:03:22.145Z] event received
  id:          6f2e...
  eventType:   Example.Items.ItemReceived
  subject:     /example/items/1
  data:
    { "itemId": "1234", "quantity": 3, "message": "hello from the example publisher" }
```

Tear down with `make tf-destroy`.

## Demo 2 — System topic (fire on resource lifecycle events)

A system topic sourced at a resource group receives a
`Microsoft.Resources.ResourceWriteSuccess` / `ResourceDeleteSuccess` event whenever a
resource inside that group is created or deleted — no publishing needed.

```bash
make consumer          # terminal 1
make tf-init-events    # terminal 2: download the providers (first run only)
make tf-apply-events
```

`terraform-system-topic/` creates the resource group, the system topic, the webhook
subscription, and then a sample ServiceBus namespace inside the group. Creating that
namespace fires an event the consumer logs:

```
────────────────────────────────────────────
[2026-09-12T00:22:06.198Z] event received
  eventType: Microsoft.Resources.ResourceWriteSuccess
  subject:   /subscriptions/.../resourceGroups/example-eventgrid-systopic/providers/Microsoft.ServiceBus/namespaces/example-trigger-ns
  data:
    { "operationName": "Microsoft.ServiceBus/namespaces/write",
      "resourceType": "Microsoft.ServiceBus/namespaces", "status": "Succeeded", ... }
```

Any resource you create in that resource group — with Terraform, `az`, or anything else
pointed at Locally — produces an event while the consumer is running. Tear down with
`make tf-destroy-events`.

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

Terraform variables live in each `*.tf` directory's `variables.tf` — region (`location`),
resource names, and `consumer_webhook_url` (default `https://localhost:8080/events`).

## Notes

- Locally's data plane is a single local process, so the webhook is reachable at
  `localhost` — no tunnel or container-host address needed.
- Locally sends the subscription-validation handshake when the subscription is created
  (during the apply), and again just before it first delivers. Start the consumer before
  you apply, so it's there to answer both.
- Locally state does not survive a restart. After restarting Locally, re-run the
  relevant `tf-apply` to recreate the resources.

## Tests

```bash
go test ./...
```
