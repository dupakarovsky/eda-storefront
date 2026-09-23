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

// publishWithRetry will try to publish a Payload type to the stream a number of times.
func publishWithRetry(
	ctx context.Context,
	publisher *foundationnats.Publisher,
	logger *slog.Logger,
	subject string,
	payload []byte,
	msgID string,
) (*jetstream.PubAck, error) {

	var lastErr error

	// kepp trying to publish to the stream until the number of attemps is exausted
	for attemp := 1; attemp <= publishMaxAttempts; attemp++ {

		attempCtx, cancel := context.WithTimeout(ctx, publishAttemptTimeout)
		// publish the event to the stream. The function should return by it's own, one way or another
		// after 1.5 seconds.
		ack, err := publisher.Publish(attempCtx, subject, payload, msgID)

		// we can then explicitly cancel for completion. Publish has has already returned no
		// not it's a no-op here.
		cancel()

		// if publishing succeeds, then return the acknowledgment.
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
			"attempt", attemp,
			"max_attempts", publishMaxAttempts,
		)

		// if this was the last attempt to publish, then give up trying
		if attemp == publishMaxAttempts {
			break
		}

		// from here we're still trying to publish to the stream.
		// we should try again after some delay
		// ------------------------------------------------------

		// define the exponential delay for the next try
		delay := publishBaseDelay * time.Duration(1<<(attemp-1))

		// wait inside the select statement before trying the loop again
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// if we're out of the loop, then it means we exausted our attemps to
	// publish and should return the last error we go when attempling to publish

	return nil, lastErr

}
