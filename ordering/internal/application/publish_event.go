package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	orderingevents "github.com/dupakarovsky/eda-storefront/ordering/internal/events"
	"github.com/google/uuid"
)

// publishEvent will take any Eventy concrete wrap it and Envelope and publish to the
// steam using the publishWithRetry function
func publishDomainEvent(ctx context.Context, publisher *foundationnats.Publisher, logger *slog.Logger, event domain.Event) error {

	// wrap the Event type into an envelope
	envelope, err := orderingevents.NewEnvelope(event)
	if err != nil {
		return fmt.Errorf("building envelope: %w", err)
	}

	// convert the envelope into JSON
	payload, err := envelope.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// define the subject under which this event will be published
	subject, err := orderingevents.SubjectFor(event)
	if err != nil {
		return fmt.Errorf("resolving subject: %w", err)
	}

	// generate a unique new message so the nats server can identify this message
	// when doing retries. Otherwise it'll treat it as 5 different messages
	msgID := uuid.NewString()

	// try publishing the message
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
