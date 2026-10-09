package foundationnats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Connect returns two things: the raw *nats.Conn (the actual socket/protocol
// connection. You'll want this later for lifecycle management, e.g. nc.Close() in shutdown)
// and a jetstream.JetStream value, which is a context for JetStream operations
// (streams, consumers, publishers) built on top of that connection, not a separate
// connection. This mirrors the split you'd expect: nats.Connect gives you the
// transport, jetstream.New gives you the higher-level API layered on it.
func Connect(cfg Config) (*nats.Conn, jetstream.JetStream, error) {

	opts := []nats.Option{
		// Client name
		nats.Name(cfg.Name),
		// With it set, nats.Connect won't fail immediately if the broker isn't
		// reachable yet; With this set, it won't let stores try to connect before
		// NATS is ready.
		nats.RetryOnFailedConnect(true),
	}

	if cfg.ConnectTimeout > 0 {
		opts = append(opts, nats.Timeout(cfg.ConnectTimeout))
	}

	// A zero value means it applies the default max reconnects attempts
	// whith is 60 times. A value of -1 is an unlimeted
	// number of reconnects.
	if cfg.MaxReconnects != 0 {
		opts = append(opts, nats.MaxReconnects(cfg.MaxReconnects))
	}

	if cfg.ReconnectWait > 0 {
		opts = append(opts, nats.ReconnectWait(cfg.ReconnectWait))
	}

	// create a new NATS connection with configuration params
	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to nats: %w", err)
	}

	// connection connection successfuly created
	//------------------------------------------

	// create a new jetstream
	js, err := jetstream.New(nc)
	if err != nil {
		// close the Connection if fails to create the stream
		nc.Close()
		return nil, nil, fmt.Errorf("creating jetstream context: %w", err)
	}

	return nc, js, nil

}
