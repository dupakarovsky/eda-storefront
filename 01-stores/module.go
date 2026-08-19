package stores

import (
	"context"
	"fmt"
	"log/slog"

	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	storesevents "github.com/dupakarovsky/eda-storefront/stores/internal/events"
	"github.com/dupakarovsky/eda-storefront/stores/internal/postgres"
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

	StoreRepo   domain.StoreRepository
	ProductRepo domain.ProductRepository
}

// New will instantiate a new Module struct for this service
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

	// initialize the database connection pool
	pool, err := foundationpostgres.NewPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("module | connecting to postgres : %w", err)
	}
	// pool is closed by the Close method in main.go

	logger.Info("🗄️ postgres pool ready",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"userl", cfg.DB.User,
		"database", cfg.DB.Database,
	)

	//============================================================================
	// NATS INITIALIZATION
	//============================================================================

	// nats configuration
	natsCfg := foundationnats.Config{
		URL:            cfg.NATS.URL,
		Name:           cfg.NATS.Name,
		ConnectTimeout: cfg.NATS.ConnectTimeout,
		MaxReconnects:  cfg.NATS.MaxReconnects,
		ReconnectWait:  cfg.NATS.ReconnectWait,
	}

	// instantiate a NATs connection and a jetstream context
	nc, js, err := foundationnats.Connect(natsCfg)
	if err != nil {
		// if NATS fails to connect, close the postgres connection
		pool.Close()
		return nil, fmt.Errorf("module | connecting to nats: %w", err)
	}
	// nats is closed by the Close method in main.go

	// success connecting
	// ------------------

	logger.Info("nats connected", "url", cfg.NATS.URL, "status", nc.Status().String())

	// create a new stream with a Name a Subects for stores
	_, err = foundationnats.EnsureStream(ctx, js, storesevents.StreamName, storesevents.StreamSubjects)
	if err != nil {
		nc.Close()
		pool.Close()
		return nil, fmt.Errorf("ensuring stores stream: %w", err)
	}

	logger.Info("🌊 stream ensured", "name", storesevents.StreamName, "subjects", storesevents.StreamSubjects)

	// instantiate a new publisher to publish to the jetstream
	publisher := foundationnats.NewPublisher(js)

	//============================================================================
	// REPOSITORIES INITIALIZATION
	//============================================================================

	storeRepo := postgres.NewStoreRepository(pool)
	productRepo := postgres.NewProductRepository(pool)

	// construct a new Module instance
	m := &Module{
		Logger:      logger,
		Pool:        pool,
		NC:          nc,
		JS:          js,
		Publisher:   publisher,
		StoreRepo:   storeRepo,
		ProductRepo: productRepo,
	}
	return m, nil
}

// Close closes both the postgres and NATS connection pools
func (m *Module) Close() {

	m.NC.Close()
	m.Logger.Info("nats connection closed")

	m.Pool.Close()
	m.Logger.Info("postgres connection closed")
}
