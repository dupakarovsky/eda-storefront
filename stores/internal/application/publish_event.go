package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	storesevents "github.com/dupakarovsky/eda-storefront/stores/internal/events"
	"github.com/google/uuid"
)

// publishDomainEvent will perform the event enveloping, marshal it into JSON
// for transport, then publish it to the stream.
func publishDomainEvent(ctx context.Context, publisher *foundationnats.Publisher, logger *slog.Logger, event domain.Event) error {

	// envelope the event
	envelope, err := storesevents.NewEnvelope(event)
	if err != nil {
		return fmt.Errorf("building envelope: %w", err)
	}

	// convert the envelope into JSON
	payload, err := envelope.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// create the subject to publish under NATS
	subject, err := storesevents.SubjectFor(event)
	if err != nil {
		return fmt.Errorf("resolving subject: %w", err)
	}

	// generates a unique ID for the message before entering publishWithRetry
	// so the exact same value is passed over to attemps 1,2 and 3. That allows the
	// jetstream to see the arrival of the same ID rather than 3 different ones.
	msgID := uuid.NewString()

	// try to publish the event using the retry function
	ack, err := publishWithRetry(ctx, publisher, logger, subject, payload, msgID)
	if err != nil {
		logger.Error("event publish exhausted retries; aggregate saved but event not delivered",
			"aggregate_id", event.AggregateID(),
			"event_name", event.EventName(),
			"subject", subject,
			"error", err,
		)

		return fmt.Errorf("publishing event: %w", err)
	}

	logger.Info("event published", "subject", subject, "stream", ack.Stream, "seq", ack.Sequence)
	return nil
}
