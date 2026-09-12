// baskets/internal/application/publish_event.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	basketsevents "github.com/dupakarovsky/eda-storefront/baskets/internal/events"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/google/uuid"
)

// publishDomainEvent will perform the event enveloping, marshal it into JSON
// for transport, then publish it to the stream.
func publishDomainEvent(ctx context.Context, publisher *foundationnats.Publisher, logger *slog.Logger, event domain.Event) error {

	// create the Envelope
	envelope, err := basketsevents.NewEnvelope(event)
	if err != nil {
		return fmt.Errorf("building envelope: %w", err)
	}

	// marshal the Envelop into json
	payload, err := envelope.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// construct the subject to be publish over
	subject, err := basketsevents.SubjectFor(event)
	if err != nil {
		return fmt.Errorf("resolving subject: %w", err)
	}

	// generate a new unique id for the message
	msgID := uuid.NewString()

	// execute the publishing with the retry function
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

	// past here the event was published

	logger.Info("event published", "subject", subject, "stream", ack.Stream, "seq", ack.Sequence)
	return nil
}
