package baskets

import (
	"context"
	"fmt"
	"log/slog"

	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"

	"github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Module defines a struct that holds every component the service needs to initialize
type Module struct {
	Logger *slog.Logger

	Pool *pgxpool.Pool
	NC   *nats.Conn
	JS   jetstream.JetStream

	Publisher  *foundationnats.Publisher
	Subscriber *foundationnats.Subscriber

	// ConnectServer *connectapi.Server
}

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

	// initialize a new pool of connections for the baskets database
	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, err
	}
	// don't call close it here. Pool must be closed in main

	logger.Info("🗄️ postgres pool ready",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"user", cfg.DB.User,
		"database", cfg.DB.Database,
	)

	//============================================================================
	// NATS INITIALIZATION
	//============================================================================

	// define a new NATS configuration
	natsCfg := foundationnats.Config{
		URL:            cfg.NATS.URL,
		Name:           cfg.NATS.Name,
		ConnectTimeout: cfg.NATS.ConnectTimeout,
		MaxReconnects:  cfg.NATS.MaxReconnects,
		ReconnectWait:  cfg.NATS.ReconnectWait,
	}

	// call Connect to recieve the raw NAST connection and the jestream context
	nc, js, err := foundationnats.Connect(natsCfg)
	if err != nil {
		// if NATS fails to connect, kill the postgres connection as well
		pool.Close()
		return nil, err
	}

	// -- TEMPORARY ----------------------
	// temporary stream name for the basket service
	const (
		storesStreamName  = "stores"
		basketsStreamName = "baskets"
	)

	// basketsStreamSubjects defines the subjects in the "baskets" stream that can be subscribed to
	var basketsStreamSubjects = []string{
		"baskets.basket.*",
	}
	// -- TEMPORARY ----------------------

	// create a new jetstream interface. the "baskets" services ensures the "baskets" stream,
	// so it owns that stream (it'll create the stream)
	_, err = foundationnats.EnsureStream(ctx, js, basketsStreamName, basketsStreamSubjects)
	if err != nil {
		// if stream fails then close all connection pools and emit an error
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring baskets stream: %w", err)
	}

	logger.Info("🌊 stream ensured", "name", basketsStreamName, "subjects", basketsStreamSubjects)

	// -----------------------
	// intialize a Publisher
	// -----------------------
	// instantiate a new publisher to publish to the jetstream
	publisher := foundationnats.NewPublisher(js)

	// -----------------------
	// intialize a Subscriber
	// -----------------------

	// start a new subscriber that subscribes to the "stores" stream. It doesn't own that
	// stream, only references it
	subscriber := foundationnats.NewSubscriber(
		"BASKETS-STORES-PROJECTOR",
		js,
		storesStreamName,
		logger,
		foundationnats.StandardConsumerConfig("baskets-stores-projector"),
		placeholderHandler(logger),
		foundationnats.PublishDeadLetter(publisher, "baskets", logger),
	)

	//============================================================================
	// NATS INITIALIZATION
	//============================================================================

	// instantiate the Module and return
	return &Module{
		Logger:     logger,
		Pool:       pool,
		NC:         nc,
		JS:         js,
		Publisher:  publisher,
		Subscriber: subscriber,
	}, nil
}

// Close will close bothe the Postgres and NATS connections
func (m *Module) Close() {
	// close nats
	m.NC.Close()
	m.Pool.Close()
}

//-----------------------------------------------------------------------------

func placeholderHandler(logger *slog.Logger) foundationnats.MessageHandler {
	return func(ctx context.Context, msg jetstream.Msg) error {
		logger.Info("received stores event (placeholder handler, no-op)",
			"subject", msg.Subject(),
		)
		return nil
	}
}
