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
	"github.com/dupakarovsky/eda-storefront/ordering"
)

func main() {

	// initiliaze the service configuration
	cfg, err := foundationconfig.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//============================================================================
	// LOGGER INITIALIZATION
	//============================================================================
	// initialize a logger for the service. Won't use a file. It'll rely on
	// docker compose logs for outputing the logs.

	logger, logFile, err := foundationlogging.New(cfg.Logger.FilePath, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	// initialize this service context
	ctx := context.Background()

	//============================================================================
	// MODULE INITIALIZATION
	//============================================================================

	// intialize the Module for the baskets service
	mod, err := ordering.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize module", "error", err)
		os.Exit(1)
	}
	defer mod.Close()

	//============================================================================
	// ROUTER INITIALIZATION
	//============================================================================
	// initialize a new router for the baskets service
	router := foundationhttp.NewRouter()

	// apply the middleware stack to the router
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(cfg.Server.RequestTimeout),
	)

	// attach the /health endpoint
	router.Handle("GET /health", healthHTTPHandler(mod))

	// construct the hander to bind the routes
	handler := router.Build()

	//============================================================================
	// SERVER INITIALIZATION AND SHUTDOWN SEQUENCE
	//============================================================================

	// initialize a new HTTP Server for the ordering service
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// intializes a new httpService for ordering
	orderingHTTPService := foundationshutdown.NewHTTPService("ORDERING-HTTP", logger, srv)

	// start the services (http + nats subscribers) and shutdown monitoring
	if err := foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, orderingHTTPService, mod.Subscribers["baskets"], mod.Subscribers["depot"]); err != nil {
		logger.Error("server exited with error", "error", err)
	}
}
