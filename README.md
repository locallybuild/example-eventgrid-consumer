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

# Example: Consume EventGrid events with a Go webhook within Locally

This example shows how to receive Azure EventGrid events in a small Go webhook consumer, using EventGrid topics provisioned with Terraform, on [Locally Build](https://locally.build).

The consumer is an HTTP server that receives EventGrid deliveries at `POST /events`, answers the subscription-validation handshake, and logs each event it receives, stamped with the time it arrived. The Go source lives in [`cmd/`](./cmd) and [`internal/`](./internal), and the Terraform that provisions the EventGrid resources lives in [`terraform/`](./terraform) and [`terraform-system-topic/`](./terraform-system-topic). A [`Makefile`](./Makefile) wraps the commands you need.

There are two demos, because "consume EventGrid events" can mean two different things:

- **Custom topic** - events your own code publishes to a topic. See Demo 1.
- **System topic** - Azure resource lifecycle events, e.g. "a ServiceBus namespace was created". This is the "fire on events happening" case. See Demo 2.

### What's here

- `cmd/consumer` - the HTTP server. Receives EventGrid deliveries at `POST /events`, answers the subscription-validation handshake, and prints each event with its arrival time. `GET /healthz` returns `ok`.
- `cmd/publisher` - sends one sample event to a custom topic (Demo 1).
- `internal/events` - EventGrid-schema event types, batch parsing and validation detection.
- `internal/consumer` - the request handler and the readable log format.
- `terraform/` - a custom topic (`azurerm_eventgrid_topic`) and a webhook subscription.
- `terraform-system-topic/` - a system topic on a resource group and a webhook subscription, plus a sample resource whose creation fires an event.

### The consumer and TLS

EventGrid (via the azurerm provider) requires the webhook URL to be `https`, and Locally verifies the certificate when it delivers. You don't need to generate one: Locally ships a `localhost` certificate its own delivery client trusts, at `~/.config/locally/localhost.pem` (and `localhost.key`). `make consumer` points the consumer at it, so this works with no extra setup. Both demos deliver to `https://localhost:8080/events`.

## Requirements

* [Locally Build](https://locally.build).
* Either [HashiCorp Terraform](https://terraform.io) or [OpenTofu](https://opentofu.org).
* Either [Docker](https://www.docker.com) or [Podman](https://podman.io) (recommended).
* [Go](https://go.dev) 1.27.4 or later, to build and run the consumer and the publisher.
* The Locally Plugin for `Microsoft.EventGrid` installed (`locally plugin install --name Microsoft.EventGrid`).
* The Locally Plugin for `Microsoft.ServiceBus` installed (`locally plugin install --name Microsoft.ServiceBus`) - Demo 2 creates a ServiceBus namespace to fire an event.

## Running the example

First up, we need to ensure our container runtime (Docker or Podman) is running, then launch Locally:

```bash
locally build --skip-browser
```

With Locally running, in another terminal we can start the consumer:

```bash
make consumer   # go run ./cmd/consumer -tls-cert ~/.config/locally/localhost.pem -tls-key ~/.config/locally/localhost.key
```

Start it before applying either demo, and leave it running: it answers the subscription-validation handshakes Locally sends.

Terraform runs through `locally run`, which points the azurerm provider at Locally and injects its credentials, so you don't configure any endpoints yourself.

> [!NOTE]
> It's possible to use OpenTofu here by running the commands the `Makefile` runs with `tofu` in place of `terraform`.

### Demo 1 - Custom topic (events you publish)

A custom topic only carries what you explicitly publish to it. In a third terminal, we can initialise Terraform, which downloads the providers we need (this is only needed on the first run), then provision the topic and its subscription, and publish a sample event:

```bash
make tf-init
make tf-apply
make publish
```

The consumer prints:

```
────────────────────────────────────────────
[2026-09-12T14:03:22.145Z] event received
  id:          6f2e...
  eventType:   Example.Items.ItemReceived
  subject:     /example/items/1
  eventTime:   2026-09-12T14:03:22Z
  dataVersion: 1.0
  data:
    {
      "itemId": "1234",
      "message": "hello from the example publisher",
      "quantity": 3
    }
```

### Demo 2 - System topic (fire on resource lifecycle events)

A system topic sourced at a resource group receives a `Microsoft.Resources.ResourceWriteSuccess` / `ResourceDeleteSuccess` event whenever a resource inside that group is created or deleted - no publishing needed:

```bash
make tf-init-events
make tf-apply-events
```

`terraform-system-topic/` creates the resource group, the system topic, the webhook subscription, and then a sample ServiceBus namespace inside the group. Creating that namespace fires an event the consumer logs:

```
────────────────────────────────────────────
[2026-09-12T00:22:06.198Z] event received
  id:          ...
  eventType:   Microsoft.Resources.ResourceWriteSuccess
  subject:     /subscriptions/.../resourceGroups/example-eventgrid-systopic/providers/Microsoft.ServiceBus/namespaces/example-trigger-ns
  ...
  data:
    {
      "operationName": "Microsoft.ServiceBus/namespaces/write",
      "resourceType": "Microsoft.ServiceBus/namespaces",
      "status": "Succeeded",
      ...
    }
```

Any resource you create in that resource group - with Terraform, `az`, or anything else pointed at Locally - produces an event while the consumer is running.

## Configuration

Consumer flags (with environment variable fallbacks):

| Flag | Env | Default |
|---|---|---|
| `-addr` | `CONSUMER_ADDR` | `:8080` |
| `-path` | `CONSUMER_PATH` | `/events` |
| `-tls-cert` | `CONSUMER_TLS_CERT` | _(none)_ |
| `-tls-key` | `CONSUMER_TLS_KEY` | _(none)_ |

With no `-tls-cert`/`-tls-key` the consumer serves plain HTTP, which is handy for poking at it with `curl`. Set both to serve HTTPS, which is what EventGrid delivery needs.

Each Terraform directory's `variables.tf` holds its variables - the region (`location`), resource names, and `consumer_webhook_url` (default `https://localhost:8080/events`).

## Notes

- Locally's data plane is a single local process, so the webhook is reachable at `localhost` - no tunnel or container-host address needed.
- Locally sends the subscription-validation handshake when the subscription is created (during the apply), and again just before it first delivers. Start the consumer before you apply, so it's there to answer both.
- Locally state does not survive a restart. After restarting Locally, re-run the relevant `tf-apply` to recreate the resources.

## Tests

```bash
make test   # go test ./...
```

## Tearing it down

```bash
make tf-destroy          # Demo 1
make tf-destroy-events   # Demo 2
```

Then stop the consumer with `Ctrl+C`.
