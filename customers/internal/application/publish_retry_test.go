package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// sets the subject in the stream to publish to
const retryTestSubject = "customers.customer.CustomerRegistered"

// payload to sent as message
var retryTestPayload = []byte(`{}`)

// fakeJetStream defines a stream to publish to
type fakeJetStream struct {
	jetstream.JetStream

	script []error
	block  bool

	mu        sync.Mutex
	calls     int
	callTimes []time.Time
	optCounts []int
}

// Publish simulates the act of publishing ot a stream. It records the the
// currnet call number, total number of calls, the time in which each all happened.
// it'sll then return a publish acknowledgement that the message was published
func (f *fakeJetStream) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {

	// begin Publishing concurrently
	f.mu.Lock()
	// copy the current number of calls on the stream
	call := f.calls
	// updates the fake stream calls to +1
	f.calls++
	// adds in the current time and options
	f.callTimes = append(f.callTimes, time.Now())
	f.optCounts = append(f.optCounts, len(opts))
	f.mu.Unlock()

	// if the fake stream stats blocking we expire the context
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// if there was any error during publishing
	if call < len(f.script) && f.script[call] != nil {
		// return the error of the call
		return nil, f.script[call]
	}

	return &jetstream.PubAck{Stream: "customers", Sequence: uint64(call + 1)}, nil
}

func newRetryHarness(fake *fakeJetStream) (*foundationnats.Publisher, *slog.Logger, *bytes.Buffer) {

	// create a new logger that stores in a buffer
	logs := new(bytes.Buffer)
	logger := slog.New(slog.NewTextHandler(logs, nil))

	// returns a fake stream, logger an logs
	return foundationnats.NewPublisher(fake), logger, logs
}

// returns a slice of durations between each call attempt and the previous
// one
func callGaps(times []time.Time) []time.Duration {
	gaps := make([]time.Duration, 0, len(times))

	for i := 1; i < len(times); i++ {
		gaps = append(gaps, times[i].Sub(times[i-1]))
	}

	return gaps
}

// warnCount returns the number of failed attemps that were printed
// to the log
func warnCount(logs *bytes.Buffer) int {
	return strings.Count(logs.String(), "publish attempt failed")
}

func TestPublishWithRetry_SucceedsOnFirstAttempt(t *testing.T) {

	synctest.Test(t, func(t *testing.T) {
		fake := &fakeJetStream{}
		publisher, logger, logs := newRetryHarness(fake)

		start := time.Now()
		ack, err := publishWithRetry(context.Background(), publisher, logger, retryTestSubject, retryTestPayload, "msg-1")
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		if ack == nil || ack.Sequence != 1 {
			t.Errorf("expected the ack of the first call, got %+v", ack)
		}
		if fake.calls != 1 {
			t.Errorf("expected exactly 1 publish call, got %d", fake.calls)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("expected no waiting on a first-attempt success, waited %s", elapsed)
		}
		if n := warnCount(logs); n != 0 {
			t.Errorf("expected no warnings, got %d", n)
		}
		if !slices.Equal(fake.optCounts, []int{1}) {
			t.Errorf("expected the message-id option on every call, got option counts %v", fake.optCounts)
		}
	})
}

func TestPublishWithRetry_RetriesTransientFailuresWithExponentialBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// stream will be down for the first 2 calls
		fake := &fakeJetStream{script: []error{errors.New("down 1"), errors.New("down 2")}}
		publisher, logger, logs := newRetryHarness(fake)

		start := time.Now()
		ack, err := publishWithRetry(context.Background(), publisher, logger, retryTestSubject, retryTestPayload, "msg-1")
		if err != nil {
			t.Fatalf("expected success on the third attempt, got error: %v", err)
		}

		// we should get an Ack at 3rd attempt (Sequence =3)
		if ack == nil || ack.Sequence != 3 {
			t.Errorf("expected the ack of the third call, got %+v", ack)
		}
		// expected to have been made 3 calls (2 fails, 1 succeeded) in total
		if fake.calls != 3 {
			t.Errorf("expected 3 publish calls, got %d", fake.calls)
		}

		wantGaps := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}

		// the gaps between 1s and 2nd calls should be of 200ms and 400ms respectivaley
		if gaps := callGaps(fake.callTimes); !slices.Equal(gaps, wantGaps) {
			t.Errorf("expected backoff gaps %v, got %v", wantGaps, gaps)
		}

		// so the ellapsed time (when 3 finally succeeds, should be 600ms)
		if elapsed := time.Since(start); elapsed != 600*time.Millisecond {
			t.Errorf("expected 600ms of total backoff, got %s", elapsed)
		}
		if n := warnCount(logs); n != 2 {
			t.Errorf("expected 2 warnings, got %d", n)
		}
	})
}

func TestPublishWithRetry_ExhaustsAttemptsAndReturnsTheLastError(t *testing.T) {

	synctest.Test(t, func(t *testing.T) {
		errs := []error{
			errors.New("down 1"),
			errors.New("down 2"),
			errors.New("down 3"),
			errors.New("down 4"),
			errors.New("down 5"),
		}
		// the fake stream will have 5 errors in total (all attempts)
		fake := &fakeJetStream{script: errs}
		publisher, logger, logs := newRetryHarness(fake)

		start := time.Now()
		// call the publishWithRetry function withour fake stream
		ack, err := publishWithRetry(context.Background(), publisher, logger, retryTestSubject, retryTestPayload, "msg-1")

		// expect no Ack back. all should have failed
		if ack != nil {
			t.Errorf("expected no ack on exhaustion, got %+v", ack)
		}
		// expect the last error to have returned
		if !errors.Is(err, errs[4]) {
			t.Errorf("expected the last attempt's error to be returned, got: %v", err)
		}
		// the number of calls made should be in total 5
		if fake.calls != publishMaxAttempts {
			t.Errorf("expected %d publish calls, got %d", publishMaxAttempts, fake.calls)
		}

		wantGaps := []time.Duration{
			200 * time.Millisecond,
			400 * time.Millisecond,
			800 * time.Millisecond,
			1600 * time.Millisecond,
		}
		if gaps := callGaps(fake.callTimes); !slices.Equal(gaps, wantGaps) {
			t.Errorf("expected backoff gaps %v, got %v", wantGaps, gaps)
		}
		// expect the total elapsed time to have been 3 seconds
		if elapsed := time.Since(start); elapsed != 3*time.Second {
			t.Errorf("expected 3s of total backoff and no sleep after the final attempt, got %s", elapsed)
		}
		if n := warnCount(logs); n != publishMaxAttempts {
			t.Errorf("expected %d warnings, got %d", publishMaxAttempts, n)
		}
	})
}

func TestPublishWithRetry_BoundsEveryAttemptWithATimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// stream will block so we should have a timeout error
		fake := &fakeJetStream{block: true}
		publisher, logger, _ := newRetryHarness(fake)

		start := time.Now()
		_, err := publishWithRetry(context.Background(), publisher, logger, retryTestSubject, retryTestPayload, "msg-1")

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected the per-attempt deadline error, got: %v", err)
		}
		if fake.calls != publishMaxAttempts {
			t.Errorf("expected %d publish calls, got %d", publishMaxAttempts, fake.calls)
		}

		wantGaps := []time.Duration{
			1700 * time.Millisecond,
			1900 * time.Millisecond,
			2300 * time.Millisecond,
			3100 * time.Millisecond,
		}
		if gaps := callGaps(fake.callTimes); !slices.Equal(gaps, wantGaps) {
			t.Errorf("expected attempt-start gaps %v, got %v", wantGaps, gaps)
		}
		if elapsed := time.Since(start); elapsed != 10500*time.Millisecond {
			t.Errorf("expected the documented 10.5s worst case, got %s", elapsed)
		}
	})
}

func TestPublishWithRetry_StopsWaitingWhenContextIsCanceledDuringBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		// stream with an error on the first try;
		fake := &fakeJetStream{script: []error{errors.New("down")}}
		publisher, logger, _ := newRetryHarness(fake)

		// create a timed context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// goroutine will cancel the context after 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		// attempt to publish; after first try fails, it'll retry again but context will be canceled
		_, err := publishWithRetry(ctx, publisher, logger, retryTestSubject, retryTestPayload, "msg-1")

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
		if fake.calls != 1 {
			t.Errorf("expected no further attempts after cancellation, got %d calls", fake.calls)
		}
		if elapsed := time.Since(start); elapsed != 100*time.Millisecond {
			t.Errorf("expected to stop at the cancellation (100ms), not wait out the 200ms backoff; elapsed %s", elapsed)
		}
	})
}

func TestPublishWithRetry_StopsWhenContextIsCanceledDuringAnAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := &fakeJetStream{block: true}
		publisher, logger, _ := newRetryHarness(fake)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err := publishWithRetry(ctx, publisher, logger, retryTestSubject, retryTestPayload, "msg-1")

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
		if fake.calls != 1 {
			t.Errorf("expected no further attempts after cancellation, got %d calls", fake.calls)
		}
		if elapsed := time.Since(start); elapsed != 100*time.Millisecond {
			t.Errorf("expected to stop at the cancellation (100ms), not at the 1.5s attempt timeout; elapsed %s", elapsed)
		}
	})
}
