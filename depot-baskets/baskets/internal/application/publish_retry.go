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

func publishWithRetry(
	ctx context.Context,
	publisher *foundationnats.Publisher,
	logger *slog.Logger,
	subject string,
	payload []byte,
	msgID string,
) (*jetstream.PubAck, error) {

	var lastErr error

	for attemp := 1; attemp <= publishMaxAttempts; attemp++ {

		// timed context for publishing to the stream. starts running now
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

		lastErr = err

		// capture the error
		logger.Warn(
			"publish attempt failed",
			"subject", subject,
			"attempt", attemp,
			"max_attempts", publishMaxAttempts,
		)

		//  if we're on the last attemp, no need to wait anymore:
		//  exit the retry and return
		if attemp == publishMaxAttempts {
			break
		}

		// from here we're still attemping to re-publish
		// ---------------------------------------------

		// add the exponential delay to then retry again
		delay := publishBaseDelay * time.Duration(1<<(attemp-1))

		// wait in the select before trying it again; once delay expires we continue looping
		// if the context expires we return the context error
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}
