package customers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/customers/internal/application"
	"github.com/dupakarovsky/eda-storefront/customers/internal/connectapi"
	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	customersevents "github.com/dupakarovsky/eda-storefront/customers/internal/events"
	"github.com/dupakarovsky/eda-storefront/customers/internal/postgres"
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

	Publisher *foundationnats.Publisher

	CustomerRepo domain.CustomerRepository

	RegisterCustomerUC  *application.RegisterCustomer
	ChangeSmsNumberUC   *application.ChangeSmsNumber
	EnableCustomerUC    *application.EnableCustomer
	DisableCustomerUC   *application.DisableCustomer
	AuthorizeCustomerUC *application.AuthorizeCustomer
	GetCustomerUC       *application.GetCustomer
	ConnectServer       *connectapi.Server
}

func New(ctx context.Context, cfg foundationconfig.Config, logger *slog.Logger) (*Module, error) {

	//============================================================================
	// POSTGRES INITIALIZATION
	//============================================================================

	// define the configuration for connecting to the postgres database
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

	// create a new pool of connections
	pool, err := foundationpostgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("module | connecting to postgres: %w", err)
	}

	logger.Info("🗄️ postgres pool ready",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"user", cfg.DB.User,
		"database", cfg.DB.Database,
	)

	//============================================================================
	// NATS INITIALIZATION
	//============================================================================

	// define a configuration for connecting to the NATS server
	natsCfg := foundationnats.Config{
		URL:            cfg.NATS.URL,
		Name:           cfg.NATS.Name,
		ConnectTimeout: cfg.NATS.ConnectTimeout,
		MaxReconnects:  cfg.NATS.MaxReconnects,
		ReconnectWait:  cfg.NATS.ReconnectWait,
	}

	// connect to the NATS server, and receive the connection object and the
	// jetstream
	nc, js, err := foundationnats.Connect(natsCfg)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("module | connecting to nats: %w", err)
	}

	logger.Info("nats connected", "url", cfg.NATS.URL, "status", nc.Status().String())

	// create the stream for the "customers" service
	_, err = foundationnats.EnsureStream(ctx, js, customersevents.StreamName, customersevents.StreamSubjects)
	if err != nil {
		// failing to create the stream, close both NATS and Postgres connections
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring customers stream: %w", err)
	}
	logger.Info("🌊 stream ensured", "name", customersevents.StreamName, "subjects", customersevents.StreamSubjects)

	// ---------------------------
	// Publisher initialization
	// --------------------------
	publisher := foundationnats.NewPublisher(js)

	//============================================================================
	// REPOSITORY INITIALIZATION
	//============================================================================
	customerRepo := postgres.NewCusomerRepository(pool)

	//============================================================================
	// USE CASES INITIALIZATION
	//============================================================================
	registerCustomerUC := application.NewRegisterCustomer(customerRepo, publisher, logger)
	changeSmsNumberUC := application.NewChangeSmsNumber(customerRepo, publisher, logger)
	enableCustomerUC := application.NewEnableCustomer(customerRepo, publisher, logger)
	disableCustomerUC := application.NewDisableCustomer(customerRepo, publisher, logger)
	authorizeCustomerUC := application.NewAuthorizeCustomer(customerRepo)
	getCustomerUC := application.NewGetCustomer(customerRepo)

	//============================================================================
	// RPC SERVER INITIALIZATION
	//============================================================================
	connectServer := connectapi.NewServer(
		registerCustomerUC,
		changeSmsNumberUC,
		enableCustomerUC,
		disableCustomerUC,
		authorizeCustomerUC,
		getCustomerUC,
	)

	return &Module{
		Logger:    logger,
		Pool:      pool,
		NC:        nc,
		JS:        js,
		Publisher: publisher,

		CustomerRepo: customerRepo,

		RegisterCustomerUC:  registerCustomerUC,
		ChangeSmsNumberUC:   changeSmsNumberUC,
		EnableCustomerUC:    enableCustomerUC,
		DisableCustomerUC:   disableCustomerUC,
		AuthorizeCustomerUC: authorizeCustomerUC,
		GetCustomerUC:       getCustomerUC,
		ConnectServer:       connectServer,
	}, nil

}

// Close will terminate all open connections (NATS and Postgres) for this service
func (m *Module) Close() {
	m.NC.Close()
	m.Logger.Info("nats connection closed")

	m.Pool.Close()
	m.Logger.Info("postgres connection closed")
}
