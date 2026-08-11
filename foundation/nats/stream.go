package foundationnats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

func EnsureStream(ctx context.Context, js jetstream.JetStream, name string, subjects []string) (jetstream.Stream, error) {

	// CreateOrUpdateStream creates a stream with the given config.
	// If stream already exists, it will be updated (if possible).
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring stream %q: %w", name, err)
	}

	return stream, nil
}
