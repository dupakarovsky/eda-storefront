package foundationnats

import (
	"context"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
)

// MessageHandler defines the signature for a function that
// handles messages from NATS
type MessageHandler func(ctx context.Context, msg jetstream.Msg) error

// Subscriber defines a type that can Subscribe to the JetStream context
// and pull (consume) messages from it via a handler.
// It will implement the foundationshutdown.Service interface, same as an httpServer
type Subscriber struct {
	name        string
	js          jetstream.JetStream
	streamName  string
	consumerCfg jetstream.ConsumerConfig
	handler     MessageHandler
	//------------------------
	mu   sync.Mutex
	cc   jetstream.ConsumeContext
	done chan struct{}
	once sync.Once
}

// NewSubscriber will construct a new Subscriber type
func NewSubscriber(
	name string,
	js jetstream.JetStream,
	streamName string,
	consumerCfg jetstream.ConsumerConfig,
	handler MessageHandler,
) *Subscriber {

	return &Subscriber{
		name:        name,
		js:          js,
		streamName:  streamName,
		consumerCfg: consumerCfg,
		handler:     handler,
		done:        make(chan struct{}),
	}
}

// Name returns the name of the subscring type
func (s *Subscriber) Name() string {
	return s.name
}

// Run will create a jetstream Consumer type, then consume messages from a stream.
// Run makes Subscriber run as a service. So it'll need to block until the Shutdown is
// called where the method will be allowed to exit, otherwise the shutdown.Run will see
// the exit and begin the closing all Service types.
func (s *Subscriber) Run() error {

	// creaete a new consumer that will consume messages from a given stream
	consumer, err := s.js.CreateOrUpdateConsumer(context.Background(), s.streamName, s.consumerCfg)
	if err != nil {
		return fmt.Errorf("ensuring consumer %q on stream %q: %w", s.consumerCfg.Durable, s.streamName, err)
	}

	// consumer was created
	// --------------------

	// call the Subscriber's handler. It'll receive the Messages from the stream
	// and act on them. It'll return a consumer Context
	cc, err := consumer.Consume(s.onMessage)
	if err != nil {
		return fmt.Errorf("starting consume for %q: %w", s.consumerCfg.Durable, err)
	}

	// now we copy the consumer Context to the Subscriber's cc field.
	// mutex-lock since this Run() call operates in a different goroutine (shutdown.Run 1st for-range)
	// than the one Shutdown()/Close() (2nd for-range) will be called, which also manipulates cc.
	s.mu.Lock()
	s.cc = cc
	s.mu.Unlock()

	// We use the same pattern as ListenAndServ(); block until told to stop.
	// Since consumer.Comsume is non-blocking, we use s channel instead
	// if Subscriber returns
	<-s.done

	return nil
}

// Shutdown will terminate the NATS Subscriber, unsubscribing it
// from the stream. Shutdown will be called by on shutdown.Run 2nd for-range
// after one for the services has error or the SIGTERM has been called.
func (s *Subscriber) Shutdown(ctx context.Context) error {

	// defines critical selection for copying the Subscriber's
	// consumer context locally
	s.mu.Lock()
	// copy the Service's context so we can operate on the copy
	cc := s.cc
	// release
	s.mu.Unlock()

	// if there's no context, we simply close the done chanel and
	// Run() is allowed to exit
	if cc == nil {
		s.markDone()
		return nil
	}

	// Drain will unsbuscribe from this stream
	cc.Drain()

	// Block on the select until the consumer.Context is close (
	// no more messages will received).
	// --------=-------------------------------------------------

	select {
	// Closed: does nothing: continues past select
	case <-cc.Closed():
	// Context expired. Return the error
	case <-ctx.Done():
		return ctx.Err()
	}

	// cc.Close() completed
	// --------------------

	// close the s.done channel so Run() can exit.
	s.markDone()
	return nil
}

// Close will ...
func (s *Subscriber) Close() error {

	// copy the Server consumer.Context locally to operate on it
	s.mu.Lock()
	cc := s.cc
	s.mu.Unlock()

	// Stop subscription and discard all messages in the buffer
	if cc != nil {
		cc.Stop()
	}

	// close the s.done channel to Run() can exit
	s.markDone()
	return nil
}

//----------------------------------------------------------------------------

// onMessage will be called to process messages when Consume is called
func (s *Subscriber) onMessage(msg jetstream.Msg) {

	// passses the message to the Subscriber's own handler function
	if err := s.handler(context.Background(), msg); err != nil {
		// if the handler fails, tell the server to re-deliver the message.
		msg.Nak()
		return
	}

	// message has been succesfuly handled
	// -----------------------------------

	// return a positive response to the server
	msg.Ack()
}

// markDone will close the Subscriber done channel.
// It uses s sync.Once to make sure it's only called ontime and never again
func (s *Subscriber) markDone() {
	s.once.Do(
		func() { close(s.done) },
	)
}
