package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/dupakarovsky/eda-storefront/customers"
	"github.com/dupakarovsky/eda-storefront/customers/customerspb/customerspbconnect"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"
	foundationlogging "github.com/dupakarovsky/eda-storefront/foundation/logging"
	foundationshutdown "github.com/dupakarovsky/eda-storefront/foundation/shutdown"
)

func main() {

	//============================================================================
	// CONFIGURATION STRUCT
	//============================================================================
	// generate the service configuraiton file
	cfg, err := foundationconfig.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//============================================================================
	// LOGGER INITIALIZATION
	//============================================================================
	// generate the structure logger for this service. close the log file when main exists
	logger, logFile, err := foundationlogging.New(cfg.Logger.FilePath, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	ctx := context.Background()

	//============================================================================
	// MODULE INITIALIZATION
	//============================================================================
	// initialize a new Module and close it when main exists
	mod, err := customers.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initilalize module", "error", err)
		os.Exit(1)
	}
	defer mod.Close()

	//============================================================================
	// ROUTER DEFINITION
	//============================================================================
	router := foundationhttp.NewRouter()
	// apply middleware stack
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(cfg.Server.RequestTimeout),
	)

	// rpc endpoint and http handler definition
	path, connectHandler := customerspbconnect.NewCustomersServiceHandler(mod.ConnectServer)
	// define endpoints
	router.Handle("GET /health", healthHTTPHandler(mod))
	router.Handle(path, connectHandler.ServeHTTP)
	// attach routes to the handler
	handler := router.Build()

	//============================================================================
	// HTTP SERVER DEFINITION
	//============================================================================
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		Handler:           handler,
	}

	// initialize the http service
	customersHTTPService := foundationshutdown.NewHTTPService("CUSTOMERS-HTTP", logger, srv)

	// start the service
	// INFO: customers has not subscriber, so it doesn't get a nats Subscriber to start as a service
	if err := foundationshutdown.Run(logger, cfg.Server.ShutdownTimeout, customersHTTPService); err != nil {
		logger.Error("server existed with error", "error", err)
	}
}
