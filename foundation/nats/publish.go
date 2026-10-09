package foundationnats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Event defines the behavior for any concrete type to implement
type Event interface {
	AggregateID() string
	EventName() string
	OccurredAt() time.Time
}

// Publisher type will hold a jetstream context and will be able
// to publish messages to the stream
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher instantiates a new Publisher type, that is able
// to publish messages to the stream.
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{
		js: js,
	}
}

// Publish will place a payload to the stream under a subject and receive back
// an Acknowledgement and a possible error.
func (p *Publisher) Publish(ctx context.Context, subject string, payload []byte, msgID string) (*jetstream.PubAck, error) {

	// publish to the stream to receive an acknowledgement.
	// jetstream.WithMsgID will set a unique UUID for this message and allow for the server
	// to identify and re-deliver this particular message if the nats server is down.
	ack, err := p.js.Publish(ctx, subject, payload, jetstream.WithMsgID(msgID))
	if err != nil {
		return nil, fmt.Errorf("publishing to %q: %w", subject, err)
	}

	// return the ack to the caller
	return ack, nil
}
