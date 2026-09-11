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
