package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// Envelope defines a type to wrap the Event concrete type for transport
type Envelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Data        json.RawMessage `json:"data"`
}

// NewEnvelope will convert the domain Eventy type to its proper Payload type,
// convert the struct into json and added to an Envelope struct to be
// published to the stream
func NewEnvelope(event domain.Event) (Envelope, error) {

	//  convert the event concrete type to it's proper Payload type
	payload, err := toPayload(event)
	if err != nil {
		return Envelope{}, err

	}

	// marshal the payload into a JSON blob
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshaling payload for %q: %w", event.EventName(), err)
	}

	// add in the JSON to the Envelope
	env := Envelope{
		AggregateID: event.AggregateID(),
		EventName:   event.EventName(),
		OccurredAt:  event.OccurredAt(),
		Data:        data,
	}

	return env, nil
}

func (e Envelope) Marshal() ([]byte, error) {

	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshaling envelope for %q: %w", e.EventName, err)
	}

	return payload, nil
}
