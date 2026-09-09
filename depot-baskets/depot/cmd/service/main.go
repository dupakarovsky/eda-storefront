package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/dupakarovsky/eda-storefront/depot"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"
	foundationloggin "github.com/dupakarovsky/eda-storefront/foundation/logging"
	foundationshutdown "github.com/dupakarovsky/eda-storefront/foundation/shutdown"
)

func main() {

	// load the  configuration from the .env file
	cfg, err := foundationconfig.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//============================================================================
	// LOGGER INITIALIZATION
	//============================================================================
	// instantiate a new logger for this service
	logger, logFile, err := foundationloggin.New(cfg.Logger.FilePath, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	//============================================================================
	// MODULE INITIALIZATION
	//============================================================================

	// appliation context
	ctx := context.Background()

	// initialize a new Module struct
	mod, err := depot.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize module", "error", err)
		os.Exit(1)
	}
	defer mod.Close()

	//============================================================================
	// ROUTER INITIALIZATION
	//============================================================================

	// instantiate a new router
	router := foundationhttp.NewRouter()

	// attach the middleware stack
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(cfg.Server.RequestTimeout),
	)
	// attach the heath route
	router.Handle("GET /health", healthHTTPHandler(mod))

	// build the router
	handler := router.Build()

	// RPC endpoint will be added later
	// path, connectHandler := depotpbconnect.NewDepotServiceHandler(mod.ConnectServer)
	// router.Handle(path, connectHandler.ServeHTTP)

	//============================================================================
	// SERVER INITIALIZATION
	//============================================================================

	// instantiate a new http server
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// intializes a new httpService for stores
	depotHTTPService := foundationshutdown.NewHTTPService("DEPOT-HTTP", logger, srv)

	// start the server and shutdown monitoring
	if err := foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, depotHTTPService, mod.Subscriber); err != nil {
		logger.Error("server exited with error", "error", err)
	}
}
