package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// Envelope will prepare a Event, properly convereted to a Payload type
// in advance, to be trasported to NATS
type Envelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	OccurredAt  time.Time       `json:"occured_at"`
	Data        json.RawMessage `json:"data"`
}

// NewEnvelope takes in a concrete Event type and wraps it into
// an Envelope struct
func NewEnvelope(event domain.Event) (Envelope, error) {

	// convert the Eventy concrete type into it's Payload type
	payload, err := toPlayload(event)
	if err != nil {
		return Envelope{}, err
	}

	// convert the payload into JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshaling payload for %q: %w", event.EventName(), err)
	}

	// wrap the data into an Envelope type
	env := Envelope{
		AggregateID: event.AggregateID(),
		EventName:   event.EventName(),
		OccurredAt:  event.OccurredAt(),
		Data:        data,
	}

	// return it
	return env, nil
}

// Marshal allows Envelope to implement JasonMarshaler interface
func (e Envelope) Marshal() ([]byte, error) {

	// convert itself into json
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshaling envelope for %q: %w", e.EventName, err)
	}

	return payload, nil
}
