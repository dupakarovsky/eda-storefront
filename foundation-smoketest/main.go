package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/foundation/postgres"
	foundationshutdown "github.com/dupakarovsky/eda-storefront/foundation/shutdown"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {

	// ---------------------------------------------------------------------------
	// CONFIG DEFINITION
	// ---------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// ---------------------------------------------------------------------------
	// LOGGER DEFINITION
	// ---------------------------------------------------------------------------
	// instantiate the app logger
	logger, file, err := logging.New("smoketest.log", cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()

	// ---------------------------------------------------------------------------
	// POSTGRES INITIALIZATION
	// ---------------------------------------------------------------------------
	ctx := context.Background()

	// define database connection parameters
	dbCfg := postgres.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		Schema:          cfg.DB.Schema,
		SSLMode:         postgres.SSLMode(cfg.DB.SSLMode),
		MaxConns:        int32(cfg.DB.MaxConns),
		MinConns:        int32(cfg.DB.MinConns),
		MaxConnLifetime: cfg.DB.MaxConnLifetime,
		MaxConnIdleTime: cfg.DB.MaxConnIdleTime,
	}

	// instatiate a new pool
	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer func() {
		pool.Close()
		logger.Info("database pool has closed")
	}()

	logger.Info("🗄️ postgres pool ready", "host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"database", cfg.DB.Database,
	)

	// ---------------------------------------------------------------------------
	// NATS INITIALIZATION
	// ---------------------------------------------------------------------------
	natsCfg := foundationnats.Config{
		URL:            cfg.NATS.URL,
		Name:           cfg.NATS.Name,
		ConnectTimeout: cfg.NATS.ConnectTimeout,
		MaxReconnects:  cfg.NATS.MaxReconnects,
		ReconnectWait:  cfg.NATS.ReconnectWait,
	}
	// get the connects and jestream
	nc, js, err := foundationnats.Connect(natsCfg)
	if err != nil {
		logger.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer func() {
		nc.Close()
		logger.Info("nats connection has closed")
	}()
	logger.Info("nats connected", "url", cfg.NATS.URL, "status", nc.Status().String())

	// get the stream
	stream, err := foundationnats.EnsureStream(ctx, js, "SMOKETEST", []string{"smoketest.>"})
	if err != nil {
		logger.Error("failed to ensure stream", "error", err)
		os.Exit(1)
	}
	logger.Info("stream ensured", "created", stream.CachedInfo().Config.Name)

	// define a new publisher
	publisher := foundationnats.NewPublisher(js)

	// ---------------------------------------------------------------------------
	// ROUTER DEFINITION
	// ---------------------------------------------------------------------------
	// instantiate a new router and apply the middlewares
	router := foundationhttp.NewRouter()
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(cfg.Server.RequestTimeout),
	)

	// Create some routes for the router

	router.Handle("GET /ping", pingHandler(logger, pool, publisher))

	router.Handle("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s", "Status OK")
	})

	router.Handle("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("oh noes...")
	})

	router.Handle("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			fmt.Fprint(w, "request completed")

		case <-r.Context().Done():
			fmt.Fprintf(w, "request timedout: %s", r.Context().Err())
		}
	})

	// ---------------------------------------------------------------------------
	// SERVER START UP
	// ---------------------------------------------------------------------------
	handler := router.Build()
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// intializes a new httpService for stores
	storesHTTPService := foundationshutdown.NewHTTPService("SMOKETEST", logger, srv)
	if err = foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, storesHTTPService); err != nil {
		logger.Error("server existed with error", "error", err)
		os.Exit(1)
	}
}

// ----------------------------------------------------------------------------

func pingHandler(logger *slog.Logger, pool *pgxpool.Pool, publisher *foundationnats.Publisher) http.HandlerFunc {

	handler := func(w http.ResponseWriter, r *http.Request) {

		// try to ping the postgres database
		if err := pool.Ping(r.Context()); err != nil {
			logger.Error("ping: postgres unreachable", "error", err)
			http.Error(w, "database unreachable", http.StatusInternalServerError)
			return
		}

		// try to publish to the stream
		ack, err := publisher.Publish(r.Context(), "smoketest.ping.received", []byte(`{"ok": true}`))
		if err != nil {
			logger.Error("publisher: nats publish failed", "error", err)
			http.Error(w, "nats publish failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"postgres":    "ok",
			"nats_stream": ack.Stream,
			"nats_seq":    ack.Sequence,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			fmt.Fprint(w, "error replying to client")
		}
	}

	return http.HandlerFunc(handler)
}
