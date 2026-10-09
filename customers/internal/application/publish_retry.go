package application

import (
	"context"
	"log/slog"
	"time"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	publishMaxAttempts    = 5
	publishBaseDelay      = 200 * time.Millisecond
	publishAttemptTimeout = 1500 * time.Millisecond
)

// publishWithRetry will try to publish a message to the straem and retry
// up to 5 time when if it fails.
func publishWithRetry(
	ctx context.Context,
	publisher *foundationnats.Publisher,
	logger *slog.Logger,
	subject string,
	payload []byte,
	msgID string,
) (*jetstream.PubAck, error) {

	var lastErr error

	// try to puslish to the jetstream
	for attempt := 1; attempt <= publishMaxAttempts; attempt++ {

		// set up a timed context of 5 seconds.
		attemptCtx, cancel := context.WithTimeout(ctx, publishAttemptTimeout)

		// call the foundationnat Publish method to publish to nats
		ack, err := publisher.Publish(attemptCtx, subject, payload, msgID)

		// we can then explicitly cancel for completion. Publish has has already returned
		// so cancel it's a no-op here.
		cancel()

		// if no error happened during the publish we can return the Ack
		if nil == err {
			return ack, nil
		}

		// past here the publishing has failed
		// ----------------------------------

		// caputure the error and publish a warning log
		lastErr = err
		logger.Warn(
			"publish attempt failed",
			"subject", subject,
			"attempt", attempt,
			"max_attempts", publishMaxAttempts,
		)

		// if this was the last attempt to publish, then give up trying
		if attempt == publishMaxAttempts {
			break
		}

		// from here we're still trying to publish to the stream.
		// we should try again after some delay
		// ------------------------------------------------------

		// define the exponential delay for the next try
		delay := publishBaseDelay * time.Duration(1<<(attempt-1))

		select {
		case <-time.After(delay):
			// when time passes, we loop again and try one more time
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}
