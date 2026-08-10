package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	foundationhttp "github.com/dupakarovsky/eda-storefront/foundation/http"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
)

func main() {

	// instantiate a new logger
	logger, file, err := logging.New("smoketest.log", "debug", "info")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()

	// instantiate a new Router
	router := foundationhttp.NewRouter()

	// Apply middlewares
	router.Use(
		foundationhttp.LoggingMiddleware(logger),
		foundationhttp.RecoveryMiddleware(logger),
		foundationhttp.TimeoutMiddleware(2*time.Second),
	)

	// Adds the route handlers
	router.Handle("/ok", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})

	router.Handle("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	router.Handle("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			fmt.Fprintf(w, "finished")
		// opt-in check for context timeout. needs to be declared so we listen
		// for the context cancellation
		case <-r.Context().Done():
			http.Error(w, "request timed out", http.StatusGatewayTimeout)
		}
	})

	/// build the handler with all middleware applied
	handler := router.Build()

	logger.Info("starting smoke test server", "addr", ":8080")
	http.ListenAndServe(":8080", handler)

}
