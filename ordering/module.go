package ordering

import (
	"context"
	"fmt"
	"log/slog"

	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/application"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/baskets"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/connectapi"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/depot"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/depotclient"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/events"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Module is a type that holds the database and message broker connection,
// repository and handlers
type Module struct {
	Logger *slog.Logger

	Pool *pgxpool.Pool
	NC   *nats.Conn
	JS   jetstream.JetStream

	Publisher   *foundationnats.Publisher
	Subscribers map[string]*foundationnats.Subscriber

	OrderRepo   domain.OrderRepository
	DepotClient domain.DepotClient

	CreateOrderUC   *application.CreateOrder
	CompleteOrderUC *application.CompleteOrder
	CancelOrderUC   *application.CancelOrder

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

	// create the ordering stream to publish it's events
	// the "ordering" services ensures "ordering" stream, so it owns that stream (it'll create the stream)
	_, err = foundationnats.EnsureStream(ctx, js, events.OrderingStreamName, events.OrderingStreamSubjects)
	if err != nil {
		// if stream fails then close all connection pools and emit an error
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring ordering stream: %w", err)
	}

	logger.Info("🌊 stream ensured", "name", events.OrderingStreamName, "subjects", events.OrderingStreamSubjects)

	// -----------------------
	// Intialize a Publisher
	// -----------------------
	// publisher is the stream the "depot" domain will publish it's events to
	publisher := foundationnats.NewPublisher(js)

	// -------------------------
	// Intialize the repository
	// -------------------------
	// initialize the repository for accessing the ordering database tables
	orderRepo := postgres.NewOrderRepository(pool)

	// -------------------------
	// Intialize the client
	// -------------------------
	// depotClient lets "ordering" synchronously orchestrate shopping list
	// creation on "depot" once an Order is created
	depotClient := depotclient.New(depotclient.Config{URL: cfg.Depot.URL})

	//============================================================================
	// USE CASE INITIALIZATION
	//============================================================================
	//needs to happen here as the Subscriber needs the createOrderUC before we
	//handle it to the RPC server
	createOrderUC := application.NewCreateOrder(orderRepo, depotClient, publisher, logger)
	completeOrderUC := application.NewCompleteOrder(orderRepo, publisher, logger)
	cancelOrderUC := application.NewCancelOrder(orderRepo, depotClient, publisher, logger)
	getOrderUC := application.NewGetOrder(orderRepo)

	// -------------------------------------------------
	// Intialize the basketsSubscriber to the "baskets" stream
	// -------------------------------------------------
	basketsSubscriber := foundationnats.NewSubscriber(
		"ORDERING-BASKETS-CHECKOUT",
		js,
		events.BasketsStreamName,
		logger,
		foundationnats.StandardConsumerConfig("ordering-baskets-checkout"),
		baskets.Handler(createOrderUC, logger),
		foundationnats.PublishDeadLetter(publisher, "ordering", logger),
	)

	// -------------------------------------------------
	// Intialize the subscriber to the "depto" stream
	// -------------------------------------------------
	depotSubscriber := foundationnats.NewSubscriber(
		"ORDERING-DEPOT-COMPLETE-SHOPPING-LIST",
		js,
		events.DepotStreamName,
		logger,
		foundationnats.StandardConsumerConfig("ordering-depot-complete-shopping-list"),
		depot.Handler(completeOrderUC, logger),
		foundationnats.PublishDeadLetter(publisher, "ordering", logger),
	)

	//============================================================================
	// CONNECTRPC SERVER INITIALIZATION
	//============================================================================

	connectServer := connectapi.NewServer(getOrderUC, cancelOrderUC)

	return &Module{
		Logger:    logger,
		Pool:      pool,
		NC:        nc,
		JS:        js,
		Publisher: publisher,
		Subscribers: map[string]*foundationnats.Subscriber{
			"baskets": basketsSubscriber,
			"depot":   depotSubscriber,
		},

		OrderRepo:   orderRepo,
		DepotClient: depotClient,

		CreateOrderUC:   createOrderUC,
		CompleteOrderUC: completeOrderUC,
		CancelOrderUC:   cancelOrderUC,

		ConnectServer: connectServer,
	}, nil
}

// Close will close the postgres and NATS connections
func (m *Module) Close() {
	m.NC.Close()
	m.Pool.Close()
}
