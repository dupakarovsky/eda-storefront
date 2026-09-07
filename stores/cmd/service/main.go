package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"
	foundationlogging "github.com/dupakarovsky/eda-storefront/foundation/logging"
	foundationshutdown "github.com/dupakarovsky/eda-storefront/foundation/shutdown"
	"github.com/dupakarovsky/eda-storefront/stores"
	"github.com/dupakarovsky/eda-storefront/stores/storespb/storespbconnect"
)

func main() {

	// load the configuration for server,logger,db and nats form .env
	cfg, err := foundationconfig.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//============================================================================
	// LOGGER INITIALIZATION
	//============================================================================
	// instantiate a new logger
	logger, logFile, err := foundationlogging.New(cfg.Logger.FilePath, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	// appliation context
	ctx := context.Background()

	//============================================================================
	// MODULE INITIALIZATION
	//============================================================================
	mod, err := stores.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize module", "error", err)
		os.Exit(1)
	}
	defer mod.Close()

	//============================================================================
	// ROUTER INITIALIZATION
	//============================================================================
	router := foundationhttp.NewRouter()

	// apply the middleware stack
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(cfg.Server.RequestTimeout),
	)

	// path and handler for the rpc server
	path, connectHandler := storespbconnect.NewStoresServiceHandler(mod.ConnectServer)

	// attach in the routes
	router.Handle("GET /health", healthHttpHandler(mod))
	router.Handle(path, connectHandler.ServeHTTP)

	// build the server's multiplex handler
	handler := router.Build()

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	//============================================================================
	// SERVER STARTUP AND GRACEFUL SHUTDOWN INITIALIZATION
	//============================================================================

	// intializes a new httpService for stores
	storesHTTPService := foundationshutdown.NewHTTPService("STORES", logger, srv)

	if err := foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, storesHTTPService); err != nil {
		logger.Error("server exited with error", "error", err)
	}
}
