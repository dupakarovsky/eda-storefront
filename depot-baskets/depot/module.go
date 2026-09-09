package depot

import (
	"context"
	"fmt"
	"log/slog"

	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Module is a type that holds the database and message borker connection,
// repository and handlers
type Module struct {
	Logger *slog.Logger

	Pool *pgxpool.Pool
	NC   *nats.Conn
	JS   jetstream.JetStream

	Publisher  *foundationnats.Publisher
	Subscriber *foundationnats.Subscriber

	// ConnectServer *connectapi.Server
}

// New will construct a new Module struct for this service
func New(ctx context.Context, cfg foundationconfig.Config, logger *slog.Logger) (*Module, error) {

	//============================================================================
	// POSTGRES INITIALIZATION
	//============================================================================

	// postgres database configuration
	dbCfg := foundationpostgres.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		Schema:          cfg.DB.Schema,
		SSLMode:         foundationpostgres.SSLMode(cfg.DB.SSLMode),
		MaxConns:        int32(cfg.DB.MaxConns),
		MinConns:        int32(cfg.DB.MinConns),
		MaxConnLifetime: cfg.DB.MaxConnLifetime,
		MaxConnIdleTime: cfg.DB.MaxConnIdleTime,
	}

	// instantiate a new postgres connection pool
	pool, err := foundationpostgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, err
	}
	// don't defer close here. It'll need to closed in main

	logger.Info("🗄️ postgres pool ready",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"user", cfg.DB.User,
		"database", cfg.DB.Database,
	)

	//============================================================================
	// NATS INITIALIZATION
	//============================================================================

	// define a new NATS configuration struct
	natsCfg := foundationnats.Config{
		URL:            cfg.NATS.URL,
		Name:           cfg.NATS.Name,
		ConnectTimeout: cfg.NATS.ConnectTimeout,
		MaxReconnects:  cfg.NATS.MaxReconnects,
		ReconnectWait:  cfg.NATS.ReconnectWait,
	}

	// initiliaze NATS, receiving the raw connection and jetstream context
	nc, js, err := foundationnats.Connect(natsCfg)
	if err != nil {
		// if NATS does not connect. kill the postgres connection as well
		pool.Close()
		return nil, err
	}

	// -- TEMPORARY ----------------------
	// temporary stream name for the depot service
	const (
		storesStreamName = "stores"
		depotStreamName  = "depot"
	)

	// depotStreamSubjects defines the subjects in the "depot" stream that can be subscribed to
	var depotStreamSubjects = []string{
		"depot.depot.*",
	}
	// -- TEMPORARY ----------------------

	// create a new jetstream interface. the "depot" services ensures "depot" stream,
	// so it owns that stream (it'll create the stream)
	_, err = foundationnats.EnsureStream(ctx, js, depotStreamName, depotStreamSubjects)
	if err != nil {
		// if stream fails then close all connection pools and emit an error
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring depot stream: %w", err)
	}

	logger.Info("🌊 stream ensured", "name", depotStreamName, "subjects", depotStreamSubjects)

	// -----------------------
	// Intialize a Publisher
	// -----------------------
	// publisher is the stream the "depot" domain will publish it's events to
	publisher := foundationnats.NewPublisher(js)

	// -----------------------
	// Intialize a Subscriber
	// -----------------------
	// start a new subscriber that subscribes to the "stores" stream. It doesn't own that
	// stream, only references it
	subscriber := foundationnats.NewSubscriber(
		"DEPOT-STORES-PROJECTOR",
		js,
		storesStreamName,
		logger,
		foundationnats.StandardConsumerConfig("depot-stores-projector"),
		placeholderHandler(logger),
		foundationnats.PublishDeadLetter(publisher, "depot", logger),
	)

	return &Module{
		Logger:     logger,
		Pool:       pool,
		NC:         nc,
		JS:         js,
		Publisher:  publisher,
		Subscriber: subscriber,
	}, nil
}

// Close will close the postgres and NATS connections
func (m *Module) Close() {
	m.NC.Close()
	m.Pool.Close()
}

// ----------------------------------------------------------------------------

// placeholderHandler function that will connect to the consumer stream and
// pull events from it.
func placeholderHandler(logger *slog.Logger) foundationnats.MessageHandler {

	return func(ctx context.Context, msg jetstream.Msg) error {
		logger.Info("received stores event (placeholder handler, no-op)",
			"subject", msg.Subject(),
		)
		return nil
	}
}
