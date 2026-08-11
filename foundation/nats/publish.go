package foundationnats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// Publisher type will hold a jetstream context
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher instantiates a new Publisher type
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{
		js: js,
	}
}

// Publish will place a payload to the stream under a subject and receive back
// an Acknowledgement and a possible error
func (p *Publisher) Publish(ctx context.Context, subject string, payload []byte) (*jetstream.PubAck, error) {

	// publish to the stream to receive an acknowledgement
	ack, err := p.js.Publish(ctx, subject, payload)
	if err != nil {
		return nil, fmt.Errorf("publishing to %q: %w", subject, err)
	}

	return ack, nil
}
