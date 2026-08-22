package application

import (
	"context"
	"log/slog"
	"time"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	publishMaxAttempts = 5
	publishBaseDelay   = 200 * time.Millisecond
	// NATS needs a timeout shorter than 5 seconds
	publishAttemptTimeout = 1500 * time.Millisecond
)

func publishWithRetry(ctx context.Context, publisher *foundationnats.Publisher, logger *slog.Logger, subject string, payload []byte) (*jetstream.PubAck, error) {

	var lastErr error

	// prepare republishing attempt till exaustion
	for attemp := 1; attemp <= publishMaxAttempts; attemp++ {

		// perform the publishing in the stream
		attempCtx, cancel := context.WithTimeout(ctx, publishAttemptTimeout)

		// Publish is now guaranteed to return within ~1.5s one way or another, because the context itself will cancel the call if nothing else does first
		ack, err := publisher.Publish(attempCtx, subject, payload)
		// deferring across loop iterations would leak publishAttemptTimeout-duration contexts stacking up until the whole function returns,
		// which is a real (if minor) resource leak worth avoiding on principle.
		cancel()

		if nil == err {
			// success, exit
			return ack, nil
		}

		// from here, this attempt failed
		// -------------------------------

		// capture the error
		lastErr = err

		// emit a warn log before retry again
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

		// add the exponential delay to then retry
		delay := publishBaseDelay * time.Duration(1<<(attemp-1))

		// wait on the select before retying again
		select {
		case <-time.After(delay):
			// wait until the dealy is over to continue to the next retry
		case <-ctx.Done():
			// if the context expires, emit the expire error
			return nil, ctx.Err()
		}

	}

	return nil, lastErr
}
