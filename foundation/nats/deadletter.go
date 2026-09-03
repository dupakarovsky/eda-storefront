package foundationnats

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

// PublishDeadLetter will construct a function that will re-publish the fail-to-deliver
// message to the stream under a different subject so this message can be pulled again later.
func PublishDeadLetter(pub *Publisher, service string, logger *slog.Logger) DeadLetterFunc {

	// fn has the same signature as a DeadLetterFunc
	fn := func(ctx context.Context, msg jetstream.Msg, cause error) {

		// construct the subject format: <service>.deadletter.<original-subject>
		// (e.g: depot.deadletter.stores.product.ProductAdded)
		subject := fmt.Sprintf("%s.deadletter.%s", service, msg.Subject())

		// emits an error log informing this message has exausted the retry-attemps
		logger.Error("message exhausted redelivery, dead-lettering",
			"subject", msg.Subject(),
			"dead_letter_subject", subject,
			"cause", cause,
		)

		// then we publish this message under a the 'dead-letter' subject so it can be
		// pulled again later
		if _, err := pub.Publish(ctx, subject, msg.Data()); err != nil {
			logger.Error("failed to publish dead letter", "subject", subject, "error", err)
		}
	}

	return fn
}
