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
