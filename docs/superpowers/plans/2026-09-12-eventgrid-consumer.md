# EventGrid Consumer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go EventGrid webhook consumer that logs each event with its arrival time, plus a Terraform directory that provisions the topic + webhook subscription against Locally, and a publisher to fire test events.

**Architecture:** A stdlib `net/http` server receives EventGrid webhook deliveries at `POST /events`, answers the subscription-validation handshake, and prints each event as a readable block stamped with server arrival time. Terraform (azurerm, run via `locally run`) creates a custom `azurerm_eventgrid_topic` and an `azurerm_eventgrid_event_subscription` whose `webhook_endpoint` points at the consumer. A small publisher POSTs a sample event to the topic so the loop is observable.

**Tech Stack:** Go 1.27 (stdlib only — `net/http`, `encoding/json`, `log/slog`, `time`, `os/signal`, `flag`), Terraform (hashicorp/azurerm `~> 4.0`), Locally (`locally run terraform`).

**Spec:** `docs/superpowers/specs/2026-09-12-eventgrid-consumer-design.md`

## Global Constraints

- Module path: `github.com/locallybuild/example-eventgrid-consumer`.
- Go stdlib only — no third-party Go dependencies.
- Event schema: **EventGrid schema** (native). Deliveries are a JSON **array** of events.
- Log output: **readable console** blocks; each event line-set includes the arrival timestamp in RFC3339 with milliseconds, UTC.
- The consumer must never panic on a single bad request; only genuinely unparseable bodies return non-2xx.
- EventGrid on this Locally install: plugin at API `2025-07-15-preview`; `topics` ✅, `eventSubscriptions` ⚠️ partial (validated in Task 5).
- Locally data plane is a single host process → webhook URL is plain `http://localhost:8080/events`.
- Terraform is always run through `locally run terraform …` so credentials/endpoints are injected; the config has no `backend`/`cloud` block (so the remote-backend gate never trips).

---

### Task 1: Module + `internal/events` (event types, parsing, validation detection)

**Files:**
- Create: `go.mod`
- Create: `internal/events/events.go`
- Test: `internal/events/events_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `const SubscriptionValidationEventType = "Microsoft.EventGrid.SubscriptionValidationEvent"`
  - `type Event struct { ID, Topic, Subject, EventType, EventTime string; Data json.RawMessage; DataVersion, MetadataVersion string }`
  - `func ParseBatch(body []byte) ([]Event, error)`
  - `func ValidationCode(e Event) (string, bool)`

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /Users/tharvey/code/src/github.com/locallybuild/example-eventgrid-consumer
go mod init github.com/locallybuild/example-eventgrid-consumer
```

- [ ] **Step 2: Write the failing test**

Create `internal/events/events_test.go`:
```go
package events

import "testing"

func TestParseBatch_Valid(t *testing.T) {
	body := []byte(`[
	  {"id":"1","topic":"/t","subject":"/s","eventType":"Example.Thing","eventTime":"2026-09-12T00:00:00Z","data":{"a":1},"dataVersion":"1.0","metadataVersion":"1"},
	  {"id":"2","eventType":"Example.Other","data":{}}
	]`)
	got, err := ParseBatch(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].ID != "1" || got[0].EventType != "Example.Thing" {
		t.Errorf("unexpected first event: %+v", got[0])
	}
}

func TestParseBatch_Malformed(t *testing.T) {
	if _, err := ParseBatch([]byte(`{not json`)); err == nil {
		t.Fatal("want error for malformed body, got nil")
	}
}

func TestValidationCode_True(t *testing.T) {
	e := Event{
		EventType: SubscriptionValidationEventType,
		Data:      []byte(`{"validationCode":"ABC-123","validationUrl":"https://x"}`),
	}
	code, ok := ValidationCode(e)
	if !ok || code != "ABC-123" {
		t.Fatalf("want (ABC-123,true), got (%q,%v)", code, ok)
	}
}

func TestValidationCode_FalseForNotification(t *testing.T) {
	e := Event{EventType: "Example.Thing", Data: []byte(`{"x":1}`)}
	if code, ok := ValidationCode(e); ok || code != "" {
		t.Fatalf("want (\"\",false), got (%q,%v)", code, ok)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/events/`
Expected: FAIL — `undefined: ParseBatch`, `undefined: Event`, etc.

- [ ] **Step 4: Write minimal implementation**

Create `internal/events/events.go`:
```go
// Package events models Azure EventGrid schema events and the webhook
// subscription-validation handshake.
package events

import (
	"encoding/json"
	"fmt"
)

// SubscriptionValidationEventType is the eventType EventGrid sends to validate
// a webhook endpoint when a subscription is created.
const SubscriptionValidationEventType = "Microsoft.EventGrid.SubscriptionValidationEvent"

// Event is a single event in the EventGrid (native) schema.
type Event struct {
	ID              string          `json:"id"`
	Topic           string          `json:"topic"`
	Subject         string          `json:"subject"`
	EventType       string          `json:"eventType"`
	EventTime       string          `json:"eventTime"`
	Data            json.RawMessage `json:"data"`
	DataVersion     string          `json:"dataVersion"`
	MetadataVersion string          `json:"metadataVersion"`
}

// ParseBatch parses an EventGrid delivery, which is a JSON array of events.
func ParseBatch(body []byte) ([]Event, error) {
	var events []Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("parsing event batch: %w", err)
	}
	return events, nil
}

// ValidationCode returns the validation code and true when e is a
// SubscriptionValidationEvent carrying a non-empty validationCode.
func ValidationCode(e Event) (string, bool) {
	if e.EventType != SubscriptionValidationEventType {
		return "", false
	}
	var data struct {
		ValidationCode string `json:"validationCode"`
	}
	if err := json.Unmarshal(e.Data, &data); err != nil || data.ValidationCode == "" {
		return "", false
	}
	return data.ValidationCode, true
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/events/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/events/
git commit -m "feat: EventGrid event types, batch parsing, validation detection"
```

---

### Task 2: `internal/consumer` (webhook handler + readable logging)

**Files:**
- Create: `internal/consumer/consumer.go`
- Test: `internal/consumer/consumer_test.go`

**Interfaces:**
- Consumes: `events.ParseBatch`, `events.ValidationCode`, `events.Event`.
- Produces:
  - `func New(log *slog.Logger, out io.Writer) *Handler`
  - `type Handler struct{ ... }` implementing `http.Handler` (has an overridable `now func() time.Time` field for tests).

- [ ] **Step 1: Write the failing test**

Create `internal/consumer/consumer_test.go`:
```go
package consumer

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestHandler(out io.Writer) *Handler {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), out)
	h.now = func() time.Time { return time.Date(2026, 9, 12, 14, 3, 22, 145_000_000, time.UTC) }
	return h
}

func TestServeHTTP_ValidationHandshake(t *testing.T) {
	h := newTestHandler(io.Discard)
	body := `[{"id":"v1","topic":"/t","eventType":"Microsoft.EventGrid.SubscriptionValidationEvent","data":{"validationCode":"CODE-42"}}]`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp struct {
		ValidationResponse string `json:"validationResponse"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.ValidationResponse != "CODE-42" {
		t.Fatalf("want validationResponse CODE-42, got %q", resp.ValidationResponse)
	}
}

func TestServeHTTP_LogsEachEventWithArrivalTime(t *testing.T) {
	var out bytes.Buffer
	h := newTestHandler(&out)
	body := `[
	  {"id":"a","subject":"/s/a","eventType":"Example.Thing","eventTime":"2026-09-12T00:00:00Z","data":{"k":"v"},"dataVersion":"1.0"},
	  {"id":"b","subject":"/s/b","eventType":"Example.Thing","data":{}}
	]`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	logged := out.String()
	for _, want := range []string{"a", "b", "2026-09-12T14:03:22.145Z", "Example.Thing"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log output missing %q\n---\n%s", want, logged)
		}
	}
}

func TestServeHTTP_MalformedBody(t *testing.T) {
	h := newTestHandler(io.Discard)
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(io.Discard)
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/consumer/`
Expected: FAIL — `undefined: New`, `undefined: Handler`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/consumer/consumer.go`:
```go
// Package consumer implements the HTTP handler that receives EventGrid webhook
// deliveries and logs each event in a human-readable form.
package consumer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/locallybuild/example-eventgrid-consumer/internal/events"
)

// Handler receives EventGrid webhook deliveries at its route.
type Handler struct {
	log *slog.Logger     // operational logs
	out io.Writer        // human-readable event blocks
	now func() time.Time // overridable for tests
}

// New returns a Handler that logs operational messages to log and writes
// readable event blocks to out.
func New(log *slog.Logger, out io.Writer) *Handler {
	return &Handler{
		log: log,
		out: out,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("reading request body", "err", err)
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	batch, err := events.ParseBatch(body)
	if err != nil {
		h.log.Error("parsing event batch", "err", err)
		http.Error(w, "invalid event batch", http.StatusBadRequest)
		return
	}

	// Handshake: if any event validates the subscription, answer it and stop.
	for _, e := range batch {
		if code, ok := events.ValidationCode(e); ok {
			h.log.Info("subscription validation handshake", "topic", e.Topic)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"validationResponse": code})
			return
		}
	}

	arrival := h.now()
	for _, e := range batch {
		h.writeEvent(arrival, e)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) writeEvent(arrival time.Time, e events.Event) {
	var b strings.Builder
	fmt.Fprintln(&b, "────────────────────────────────────────────")
	fmt.Fprintf(&b, "[%s] event received\n", arrival.Format("2006-01-02T15:04:05.000Z07:00"))
	fmt.Fprintf(&b, "  id:          %s\n", e.ID)
	fmt.Fprintf(&b, "  eventType:   %s\n", e.EventType)
	fmt.Fprintf(&b, "  subject:     %s\n", e.Subject)
	fmt.Fprintf(&b, "  eventTime:   %s\n", e.EventTime)
	fmt.Fprintf(&b, "  dataVersion: %s\n", e.DataVersion)
	fmt.Fprintf(&b, "  data:\n%s\n", indentJSON(e.Data, "    "))
	_, _ = io.WriteString(h.out, b.String())
}

func indentJSON(raw json.RawMessage, prefix string) string {
	if len(raw) == 0 {
		return prefix + "(no data)"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, prefix, "  "); err != nil {
		return prefix + string(raw) // not valid JSON; print raw
	}
	return prefix + buf.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/consumer/`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
git add internal/consumer/
git commit -m "feat: webhook handler with validation handshake and readable event logging"
```

---

### Task 3: `cmd/consumer` (entrypoint: flags, routing, graceful shutdown)

**Files:**
- Create: `cmd/consumer/main.go`

**Interfaces:**
- Consumes: `consumer.New(log, out)`.
- Produces: the `consumer` binary. Flags/env: `-addr`/`CONSUMER_ADDR` (default `:8080`), `-path`/`CONSUMER_PATH` (default `/events`).

- [ ] **Step 1: Write the entrypoint**

Create `cmd/consumer/main.go`:
```go
// Command consumer runs an HTTP server that receives Azure EventGrid webhook
// deliveries and logs each event with its arrival time.
package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/locallybuild/example-eventgrid-consumer/internal/consumer"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", envOr("CONSUMER_ADDR", ":8080"), "listen address")
	path := flag.String("path", envOr("CONSUMER_PATH", "/events"), "webhook path (must start with /)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	h := consumer.New(log, os.Stdout)

	mux := http.NewServeMux()
	mux.Handle(*path, h)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Info("consumer listening", "addr", *addr, "path", *path)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}
```

- [ ] **Step 2: Build and vet**

Run: `go build ./... && go vet ./...`
Expected: no output (success).

- [ ] **Step 3: Smoke-test the handshake and a notification locally (no Locally needed)**

Run the server, then in a second shell exercise both paths:
```bash
go run ./cmd/consumer &
sleep 1
# validation handshake -> should print {"validationResponse":"CODE-42"}
curl -s -X POST localhost:8080/events \
  -d '[{"eventType":"Microsoft.EventGrid.SubscriptionValidationEvent","data":{"validationCode":"CODE-42"}}]'
echo
# notification -> consumer should print a readable event block with an arrival time
curl -s -X POST localhost:8080/events \
  -d '[{"id":"x","subject":"/s","eventType":"Example.Thing","eventTime":"2026-09-12T00:00:00Z","data":{"hello":"world"},"dataVersion":"1.0"}]'
curl -s localhost:8080/healthz; echo
kill %1
```
Expected: the curl prints `{"validationResponse":"CODE-42"}`; the consumer stdout shows a readable event block containing `hello`, `world`, and an arrival timestamp; `/healthz` prints `ok`.

- [ ] **Step 4: Commit**

```bash
git add cmd/consumer/
git commit -m "feat: consumer entrypoint with flags, health check, graceful shutdown"
```

---

### Task 4: Terraform config (topic + webhook subscription)

**Files:**
- Create: `terraform/providers.tf`
- Create: `terraform/variables.tf`
- Create: `terraform/main.tf`
- Create: `terraform/outputs.tf`

**Interfaces:**
- Consumes: nothing from Go code.
- Produces: Terraform outputs `topic_endpoint` (string) and `topic_primary_access_key` (string, sensitive), consumed by the publisher in Task 6.

- [ ] **Step 1: Write `providers.tf`**

```hcl
terraform {
  required_version = ">= 1.6"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azurerm" {
  features {}

  # Terraform is run via `locally run terraform ...`, which injects
  # ARM_* / AZURE_* credentials and endpoints. Locally does not implement
  # resource-provider registration, so skip it.
  resource_provider_registrations = "none"
}
```
> If apply reports the `resource_provider_registrations` attribute is unknown (older azurerm), replace that line with `skip_provider_registration = true`.

- [ ] **Step 2: Write `variables.tf`**

```hcl
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
```

- [ ] **Step 3: Write `main.tf`**

```hcl
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
```

- [ ] **Step 4: Write `outputs.tf`**

```hcl
output "topic_endpoint" {
  description = "Endpoint to publish events to."
  value       = azurerm_eventgrid_topic.example.endpoint
}

output "topic_primary_access_key" {
  description = "Access key for publishing (aeg-sas-key header)."
  value       = azurerm_eventgrid_topic.example.primary_access_key
  sensitive   = true
}
```

- [ ] **Step 5: Format and validate the HCL (no instance needed)**

Run:
```bash
locally run terraform -chdir=terraform fmt
locally run terraform -chdir=terraform init -backend=false
locally run terraform -chdir=terraform validate
```
Expected: `fmt` leaves files formatted, `validate` reports "Success! The configuration is valid." (`fmt`/`validate`/`init -backend=false` are exempt from the remote-backend gate.)

- [ ] **Step 6: Commit**

```bash
git add terraform/ .gitignore
git commit -m "feat: terraform for EventGrid topic and webhook subscription"
```

---

### Task 5: Locally integration spike — GATE (confirm end-to-end delivery)

This task is a **spike**, not TDD: it validates the risky unknowns against a running Locally instance before we invest in the publisher/docs. If any check fails and can't be resolved by the documented fallbacks, **stop and report** rather than working around it.

**Files:** none created (may edit `terraform/*.tf` or the webhook URL based on findings; commit any change made).

- [ ] **Step 1: Start Locally**

Run: `locally build --skip-browser` (or use the `locally-lifecycle` skill). Wait until it reports ready. Then confirm the region default is valid: `locally regions list` — if `berlin` is absent, set `-var location=<a listed region>` on subsequent applies (or change the `variables.tf` default).

- [ ] **Step 2: Start the consumer**

Run in its own shell: `go run ./cmd/consumer` (listening on `:8080`). Leave it running so it can answer the validation handshake at subscription-create time.

- [ ] **Step 3: Apply the Terraform**

Run:
```bash
locally run terraform -chdir=terraform init
locally run terraform -chdir=terraform apply -auto-approve
```
Watch for and resolve, in order:
- **API version rejected** (`NoRegisteredProviderFound … supported api-versions are …`): the azurerm-selected version isn't served. Note the supported versions from the error; this is a Locally/azurerm mismatch — record it and report (do not hack around it).
- **`http` webhook rejected** (EventGrid may require `https`): if so, note it — resolving means either Locally accepts `http` for localhost (preferred) or the consumer must serve TLS. Record the exact error and report before proceeding.
- **`eventSubscriptions` ⚠️ partial** manifests as the subscription create failing while the topic succeeds. Capture the error; if the generic form is unsupported, report it (a nested-subscription workaround is out of scope for this pass).
- **Handshake**: if the consumer logs `subscription validation handshake`, Locally performs create-time validation and the consumer satisfied it. If not, note that Locally skipped it.

- [ ] **Step 4: Publish a test event manually and confirm delivery**

Run (reads outputs, POSTs a one-element array with the SAS key header):
```bash
ENDPOINT=$(locally run terraform -chdir=terraform output -raw topic_endpoint)
KEY=$(locally run terraform -chdir=terraform output -raw topic_primary_access_key)
curl -s -X POST "$ENDPOINT" -H "aeg-sas-key: $KEY" -H "Content-Type: application/json" \
  -d '[{"id":"spike-1","subject":"/spike","eventType":"Example.Thing","eventTime":"2026-09-12T00:00:00Z","data":{"hello":"world"},"dataVersion":"1.0"}]'
```
Expected: the consumer's stdout prints a readable event block for `spike-1` with an arrival timestamp. **This is the go/no-go**: delivery working confirms the whole design.

- [ ] **Step 5: Record findings and commit any config change**

Append a short "Spike findings" note to the spec (`docs/superpowers/specs/2026-09-12-eventgrid-consumer-design.md`, §10) recording: chosen region, http-vs-https outcome, whether the handshake ran, and the working `eventSubscriptions` form. Commit any `.tf` change:
```bash
git add terraform/ docs/
git commit -m "chore: record Locally EventGrid spike findings"
```

---

### Task 6: `cmd/publisher` (fire a sample event)

**Files:**
- Create: `cmd/publisher/main.go`
- Test: `cmd/publisher/main_test.go`

**Interfaces:**
- Consumes: Terraform outputs `topic_endpoint`, `topic_primary_access_key` (via flags/env `TOPIC_ENDPOINT`/`TOPIC_KEY`).
- Produces: the `publisher` binary; internal `buildPayload(subject, eventType string) ([]byte, string, error)` returning the JSON array body and the generated event id.

- [ ] **Step 1: Write the failing test**

Create `cmd/publisher/main_test.go`:
```go
package main

import (
	"encoding/json"
	"testing"
)

func TestBuildPayload(t *testing.T) {
	body, id, err := buildPayload("/example/items/1", "Example.Items.ItemReceived")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("want non-empty event id")
	}
	var events []map[string]any
	if err := json.Unmarshal(body, &events); err != nil {
		t.Fatalf("payload is not a JSON array: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	e := events[0]
	if e["id"] != id {
		t.Errorf("want id %q in payload, got %v", id, e["id"])
	}
	for _, k := range []string{"id", "subject", "eventType", "eventTime", "dataVersion", "data"} {
		if _, ok := e[k]; !ok {
			t.Errorf("payload missing required field %q", k)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/publisher/`
Expected: FAIL — `undefined: buildPayload`.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/publisher/main.go`:
```go
// Command publisher POSTs a sample EventGrid-schema event to a topic endpoint
// so the consumer can be seen receiving it end-to-end.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func buildPayload(subject, eventType string) (body []byte, id string, err error) {
	id = newID()
	event := map[string]any{
		"id":          id,
		"subject":     subject,
		"eventType":   eventType,
		"eventTime":   time.Now().UTC().Format(time.RFC3339),
		"dataVersion": "1.0",
		"data": map[string]any{
			"itemId":   "1234",
			"quantity": 3,
			"message":  "hello from the example publisher",
		},
	}
	body, err = json.Marshal([]any{event})
	return body, id, err
}

func main() {
	endpoint := flag.String("endpoint", os.Getenv("TOPIC_ENDPOINT"), "EventGrid topic endpoint URL")
	key := flag.String("key", os.Getenv("TOPIC_KEY"), "EventGrid topic access key (aeg-sas-key)")
	subject := flag.String("subject", "/example/items/1", "event subject")
	eventType := flag.String("type", "Example.Items.ItemReceived", "event type")
	flag.Parse()

	if *endpoint == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "endpoint and key are required (flags, or TOPIC_ENDPOINT/TOPIC_KEY env)")
		os.Exit(2)
	}

	body, id, err := buildPayload(*subject, *eventType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "building payload: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "building request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("aeg-sas-key", *key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "publishing: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("published event %s -> %s\n", id, resp.Status)
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/publisher/`
Expected: PASS.

- [ ] **Step 5: End-to-end check via the publisher binary**

With Locally, the consumer, and an applied topic all up (from Task 5):
```bash
TOPIC_ENDPOINT=$(locally run terraform -chdir=terraform output -raw topic_endpoint) \
TOPIC_KEY=$(locally run terraform -chdir=terraform output -raw topic_primary_access_key) \
go run ./cmd/publisher
```
Expected: prints `published event <id> -> 200 OK`; the consumer logs a readable block for that id.

- [ ] **Step 6: Commit**

```bash
git add cmd/publisher/
git commit -m "feat: sample event publisher"
```

---

### Task 7: Makefile + README

**Files:**
- Create: `Makefile`
- Create: `README.md`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: developer entrypoints and docs.

- [ ] **Step 1: Write the `Makefile`**

```makefile
.PHONY: build test consumer tf-init tf-apply tf-destroy publish

build:
	go build ./...

test:
	go test ./...

consumer:
	go run ./cmd/consumer

tf-init:
	locally run terraform -chdir=terraform init

tf-apply:
	locally run terraform -chdir=terraform apply

tf-destroy:
	locally run terraform -chdir=terraform destroy

publish:
	TOPIC_ENDPOINT=$$(locally run terraform -chdir=terraform output -raw topic_endpoint) \
	TOPIC_KEY=$$(locally run terraform -chdir=terraform output -raw topic_primary_access_key) \
	go run ./cmd/publisher
```

- [ ] **Step 2: Write the `README.md`**

Include: what it is; prerequisites (Go, Locally running, EventGrid plugin — `locally plugin supports Microsoft.EventGrid/topics`); the run sequence (1) `go run ./cmd/consumer`, (2) `make tf-apply`, (3) `make publish`; a note that Locally state is ephemeral (re-run `make tf-apply` after a Locally restart); and any spike findings that affect usage (e.g. region, http/https). Keep prose plain (run it past the `humanizer` skill).

- [ ] **Step 3: Verify the full build and tests pass**

Run: `go build ./... && go test ./...`
Expected: all packages build; all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add Makefile README.md
git commit -m "docs: Makefile and README for running the example end-to-end"
```

---

## Self-Review

**Spec coverage:** Consumer webhook + handshake (Tasks 2,3) ✓; EventGrid schema types + batch parse (Task 1) ✓; readable log with arrival time (Task 2) ✓; Terraform topic + webhook subscription run via `locally run` (Task 4) ✓; publisher (Task 6) ✓; Makefile/README (Task 7) ✓; the integration spike / go-no-go for the ⚠️ `eventSubscriptions`, http-vs-https, handshake and delivery unknowns (Task 5) ✓; ephemeral-state note (Task 7 README) ✓. No spec requirement is left without a task.

**Placeholder scan:** No TBD/TODO. All code steps contain full code; the spike task lists concrete commands and named fallbacks rather than "handle errors".

**Type consistency:** `events.Event`, `events.ParseBatch`, `events.ValidationCode`, `SubscriptionValidationEventType` are defined in Task 1 and consumed unchanged in Task 2. `consumer.New(*slog.Logger, io.Writer) *Handler` defined in Task 2, consumed in Task 3. `buildPayload(string, string) ([]byte, string, error)` defined and tested in Task 6. Terraform outputs `topic_endpoint` / `topic_primary_access_key` produced in Task 4 and consumed in Tasks 5–7.
