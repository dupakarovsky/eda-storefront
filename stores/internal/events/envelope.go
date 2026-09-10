package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// Envelope will prepare a Event, properly convereted to a Payload type
// in advance, to be trasported to NATS
type Envelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Data        json.RawMessage `json:"data"`
}

func NewEnvelope(event domain.Event) (Envelope, error) {

	// first, convert the Event concrete type into a proper Payload type
	payload, err := toPayload(event)
	if err != nil {
		return Envelope{}, err
	}

	// then marshal the payload into JSON bytes
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshaling envelope for %q: %w", event.EventName(), err)
	}

	// construct the Envelope type with Data carried the Payload type as JSON bytes.
	env := Envelope{
		AggregateID: event.AggregateID(),
		EventName:   event.EventName(),
		OccurredAt:  event.OccurredAt(),
		Data:        data,
	}

	return env, nil
}

// Marshal allows Envelope to use the Marshaler for the entire wrapper.
// the e.Data is already a raw JSON when this gets called.
func (e Envelope) Marshal() ([]byte, error) {

	// convert the entire Envelope type into JSON.
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshaling envelope for %q: %w", e.EventName, err)
	}

	return payload, nil
}
