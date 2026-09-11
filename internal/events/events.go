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
