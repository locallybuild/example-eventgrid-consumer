# EventGrid Consumer — Design

- **Date:** 2026-09-12
- **Status:** Approved for planning
- **Author:** tharvey (claude2@tom.ibuildstuff.eu) + Claude

## 1. Goal

Build a small, runnable **example** Go project that consumes Azure EventGrid
events and logs the contents of each event together with the time it arrived,
plus a Terraform directory that provisions the EventGrid topic + subscription so
events can be fired and observed end-to-end. Target environment is **Locally**
(the local Azure emulator), so the whole loop runs on the dev machine with no
cloud account.

## 2. Context & constraints (verified)

Checked against the running Locally install (`locally plugin supports ...`):

- `Microsoft.EventGrid` is **supported** via a plugin at API version
  `2025-07-15-preview` (the binary is a `local-build`; the plugin is not in the
  public catalogue — treat it as in-development on this machine).
- Usable types: `topics` ✅, `topics/eventSubscriptions` ✅, `systemTopics` ✅,
  `domains` ✅, `namespaces` ✅, `partnerTopics` ✅.
- `Microsoft.EventGrid/eventSubscriptions` (the generic/standalone form) is
  **⚠️ partially supported**. Terraform's `azurerm_eventgrid_event_subscription`
  may create the subscription as this generic extension resource rather than the
  topic-nested `topics/eventSubscriptions`. **This is the main risk** and is
  resolved by the spike in §8 before the rest is built out.
- Locally's data plane runs as a **single host Go process, not in a container**
  (confirmed by the user). So the webhook is reachable at a plain
  `http://localhost:PORT` / `http://127.0.0.1:PORT` URL — no `host.docker.internal`
  or LAN-IP workaround.
- Locally data-plane state is **ephemeral across restarts**; after a Locally
  restart the Terraform must be re-applied to recreate the topic/subscription.

## 3. Decisions

| Decision | Choice |
|---|---|
| Delivery pattern | Classic **custom topic + webhook push** subscription |
| Event schema | **EventGrid schema** (native), events delivered as a JSON array |
| Log output | **Readable console** block per event, including arrival time |
| Demo publisher | **Included** — fires a sample event so the loop is observable |
| Webhook URL | Plain `http://localhost:8080/events` (Locally is a host process) |

## 4. Non-goals (YAGNI)

No persistence/database, no CloudEvents schema, no dead-letter handling, no
auth on the consumer beyond EventGrid's own delivery, no metrics/tracing, no
multiple topics or filters beyond a basic example. This is a demonstration
consumer, kept small and readable.

## 5. Architecture

```
cmd/consumer/main.go     — webhook HTTP server entrypoint (flags/env, wiring, graceful shutdown)
cmd/publisher/main.go    — publishes a sample EventGrid event to the topic
internal/events/         — EventGrid event types, validation-event detection, batch parsing
internal/consumer/       — HTTP handler + readable event logging
terraform/               — providers.tf, main.tf, variables.tf, outputs.tf
Makefile                 — build / run-consumer / tf-apply / publish targets
README.md                — end-to-end run instructions
```

Each unit has one purpose and a clear interface: `internal/events` knows the
wire format and nothing about HTTP; `internal/consumer` knows HTTP and delegates
parsing to `internal/events`; `cmd/*` only wire flags/env to those packages.

## 6. Data flow

```
publisher ──POST array──▶ topic endpoint (Locally EventGrid data plane)
                                │  matches subscription
                                ▼
                         POST JSON array ──▶ consumer  POST /events
                                                 │ stamp arrival time
                                                 │ log each event (readable)
                                                 ▼ 200 (fast)
```

On subscription creation, EventGrid (and Locally, if it implements the
handshake) first sends a **SubscriptionValidationEvent**; the consumer answers
it synchronously (see §7.2).

## 7. Component detail

### 7.1 `internal/events`

EventGrid-schema event:

```go
type Event struct {
    ID              string          `json:"id"`
    Topic           string          `json:"topic"`
    Subject         string          `json:"subject"`
    EventType       string          `json:"eventType"`
    EventTime       string          `json:"eventTime"`   // RFC3339 as sent
    Data            json.RawMessage `json:"data"`
    DataVersion     string          `json:"dataVersion"`
    MetadataVersion string          `json:"metadataVersion"`
}
```

- `ParseBatch([]byte) ([]Event, error)` — events arrive as a JSON **array**.
- `IsValidation(Event) (code string, ok bool)` — true when
  `EventType == "Microsoft.EventGrid.SubscriptionValidationEvent"`; pulls
  `data.validationCode`.
- The validation event itself also arrives as a single-element array.

### 7.2 `internal/consumer`

`POST /events` handler:

1. Read + parse the body as a batch.
2. If any event is a validation event → respond `200` with
   `{"validationResponse": "<validationCode>"}` and return. (EventGrid also sets
   header `aeg-event-type: SubscriptionValidation`; we discriminate on
   `eventType` so it works regardless of whether Locally sends that header.)
3. Otherwise capture `arrival := time.Now().UTC()`, log each event, respond
   `200` promptly. Non-2xx makes EventGrid retry, so only genuinely unparseable
   bodies return `400`.

`GET /healthz` → `200 ok`. Unexpected methods/paths → `404`/`405`. A single bad
request never panics the server.

**Readable log block** (to stdout), one per event:

```
────────────────────────────────────────────
[2026-09-12T14:03:22.145Z] event received
  id:          6f2e...
  eventType:   Contoso.Items.ItemReceived
  subject:     /items/1234
  eventTime:   2026-09-12T14:03:22.101Z
  dataVersion: 1.0
  data:
    {
      "itemId": "1234",
      "quantity": 3
    }
```

Arrival time is server receipt time in RFC3339 with milliseconds, UTC. `data` is
pretty-indented; if it is not valid JSON it is printed raw. Operational logs
(startup, shutdown, parse errors) use `log/slog`; event blocks use a dedicated
formatter so they stay readable.

### 7.3 `cmd/consumer`

Flags with env fallback: `-addr` (`CONSUMER_ADDR`, default `:8080`), `-path`
(`CONSUMER_PATH`, default `/events`). Starts `http.Server`, shuts down
gracefully on SIGINT/SIGTERM.

### 7.4 `cmd/publisher`

Reads topic endpoint + access key from flags/env (populated from Terraform
outputs). Builds a sample `Event` (fresh `id`, `eventTime`, example `data`),
POSTs `[event]` with header `aeg-sas-key: <key>`, prints the response status.
Lets us "fire an event" and watch the consumer log it.

## 8. Testing strategy

**TDD unit tests (write first):**

- `internal/events`: `ParseBatch` on a valid array; on a single validation
  event; on malformed JSON (error). `IsValidation` true/false + code extraction.
- `internal/consumer` via `httptest`: validation event → `200` +
  `{"validationResponse": code}`; a 2-event batch → `200` and both events
  logged, each with an arrival timestamp (assert against a captured log buffer);
  malformed body → `400`; `/healthz` → `200`.

**Integration spike (first implementation step, before building publisher/README):**

1. Point azurerm at Locally (via the `locally-run` skill), `terraform apply` a
   topic + webhook subscription with URL `http://localhost:8080/events`.
2. Run the consumer; confirm whether Locally performs the validation handshake
   at create time (and that the consumer satisfies it).
3. Publish one event; confirm Locally **delivers** the webhook and the consumer
   logs it.
4. Settle the ⚠️ `eventSubscriptions` shape (nested vs generic) and adjust the
   Terraform accordingly.

If the spike surfaces a Locally gap (e.g. no webhook delivery), stop and report
before building further — do not paper over it.

## 9. Terraform design

- `providers.tf` — `azurerm` configured for Locally per the `locally-run` skill
  (exact endpoint/auth config confirmed during the spike). `features {}` block.
- `main.tf` — `azurerm_resource_group` → `azurerm_eventgrid_topic` →
  `azurerm_eventgrid_event_subscription` with
  `webhook_endpoint { url = var.consumer_webhook_url }`.
- `variables.tf` — `location`, `resource_group_name`, `topic_name`,
  `subscription_name`, `consumer_webhook_url` (default
  `http://localhost:8080/events`).
- `outputs.tf` — `topic_endpoint` and `topic_primary_access_key` (sensitive),
  consumed by the publisher.

## 10. Risks / open questions

1. `eventSubscriptions` is ⚠️ partial on Locally — resolved by the spike (§8).
2. Whether Locally runs the create-time validation handshake — consumer handles
   both cases; spike confirms which.
3. Exact azurerm-against-Locally provider config + topic endpoint URL form —
   resolved via the `locally-run` skill during the spike.
4. Locally state is ephemeral — README notes: re-apply after a Locally restart.

## 11. Build / run outline (for the README)

1. `locally` running; `terraform apply` in `terraform/`.
2. `go run ./cmd/consumer` (listens on `:8080`).
3. `go run ./cmd/publisher` (reads TF outputs) — watch the consumer log the event.
