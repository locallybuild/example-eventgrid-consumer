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
