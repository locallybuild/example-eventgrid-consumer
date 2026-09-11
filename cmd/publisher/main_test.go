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
