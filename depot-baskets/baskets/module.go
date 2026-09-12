package baskets

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/application"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/connectapi"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/events"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/postgres"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"

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

	CatalogRepo catalog.Repository

	StartBasketUC  *application.StartBasket
	AddItemUC      *application.AddItem
	RemoveItemUC   *application.RemoveItem
	CheckoutUC     *application.Checkout
	CancelBasketUC *application.CancelBasket
	GetBasketUC    *application.GetBasket

	ConnectServer *connectapi.Server
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
	pool, err := foundationpostgres.NewPool(ctx, dbCfg)
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
	// REPOSITORY INITIALIZATION
	//============================================================================
	catalogRepo := postgres.NewCatalogRepository(pool)
	basketRepo := postgres.NewBasketRepository(pool)

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

	// create a new jetstream interface. the "baskets" services ensures the "baskets" stream,
	// so it owns that stream (it'll create the stream)
	_, err = foundationnats.EnsureStream(ctx, js, events.BasketsStreamName, events.BasketsStreamSubjects)
	if err != nil {
		// if stream fails then close all connection pools and emit an error
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring baskets stream: %w", err)
	}

	logger.Info("🌊 stream ensured", "name", events.BasketsStreamName, "subjects", events.BasketsStreamSubjects)

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
		events.StoresStreamName,
		logger,
		foundationnats.StandardConsumerConfig("baskets-stores-projector"),
		catalog.Project(catalogRepo, logger),
		foundationnats.PublishDeadLetter(publisher, "baskets", logger),
	)

	//============================================================================
	// APPLICATION USE CASES INITIALIZATION
	//============================================================================

	startBasketUC := application.NewStartBasket(basketRepo, publisher, logger)
	addItemUC := application.NewAddItem(basketRepo, catalogRepo, publisher, logger)
	removeItemUC := application.NewRemoveItem(basketRepo, publisher, logger)
	checkoutUC := application.NewCheckout(basketRepo, publisher, logger)
	cancelBasketUC := application.NewCancelBasket(basketRepo, publisher, logger)
	getBasketUC := application.NewGetBasket(basketRepo)

	//============================================================================
	// CONNECTRPC SERVER INITIALIZATION
	//============================================================================

	connectServer := connectapi.NewServer(startBasketUC, addItemUC, removeItemUC, checkoutUC, cancelBasketUC, getBasketUC)

	//============================================================================
	// MODULE INITIALIZATION
	//============================================================================

	// instantiate the Module and return
	return &Module{
		Logger:     logger,
		Pool:       pool,
		NC:         nc,
		JS:         js,
		Publisher:  publisher,
		Subscriber: subscriber,

		CatalogRepo: catalogRepo,

		StartBasketUC:  startBasketUC,
		AddItemUC:      addItemUC,
		RemoveItemUC:   removeItemUC,
		CheckoutUC:     checkoutUC,
		CancelBasketUC: cancelBasketUC,
		GetBasketUC:    getBasketUC,

		ConnectServer: connectServer,
	}, nil
}

// Close will close bothe the Postgres and NATS connections
func (m *Module) Close() {
	// close nats
	m.NC.Close()
	m.Pool.Close()
}

//-----------------------------------------------------------------------------
