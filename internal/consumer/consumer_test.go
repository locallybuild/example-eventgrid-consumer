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
