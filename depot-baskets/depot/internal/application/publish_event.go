// depot/internal/application/publish_event.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	depotevents "github.com/dupakarovsky/eda-storefront/depot/internal/events"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/google/uuid"
)

// publishDomainEvent will perform the event enveloping, marshal it into JSON
// for transport, then publish it to the stream.
func publishDomainEvent(ctx context.Context, publisher *foundationnats.Publisher, logger *slog.Logger, event domain.Event) error {

	// envelop the event
	envelope, err := depotevents.NewEnvelope(event)
	if err != nil {
		return fmt.Errorf("building envelope: %w", err)
	}

	// convert to JSON
	payload, err := envelope.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// construct the subject string to publish under
	subject, err := depotevents.SubjectFor(event)
	if err != nil {
		return fmt.Errorf("resolving subject: %w", err)
	}

	// generate a unique UUID for this message, so NATS can identify it if it
	// needst to relideliver it
	msgID := uuid.NewString()

	// publish this message to the stream under the 'subejct'
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
