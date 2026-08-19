package main

import (
	"fmt"
	"net/http"

	"github.com/dupakarovsky/eda-storefront/stores"
	"github.com/nats-io/nats.go"
)

func healthHandler(mod *stores.Module) http.HandlerFunc {

	// new handler
	h := func(w http.ResponseWriter, r *http.Request) {

		// try connect to the database
		err := mod.Pool.Ping(r.Context())
		if err != nil {
			mod.Logger.Error("health: postgres unreachable", "error", err)
			http.Error(w, "database unreachable", http.StatusInternalServerError)
			return
		}

		// try connecting to nats
		if mod.NC.Status() != nats.CONNECTED {
			mod.Logger.Error("health: nats not connected", "status", mod.NC.Status().String())
			http.Error(w, "nats not connected", http.StatusInternalServerError)
			return
		}

		// both connections successful
		// ---------------------------

		// write the status code
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	}

	// create a http handler function
	return http.HandlerFunc(h)
}
