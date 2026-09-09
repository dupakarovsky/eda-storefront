package main

import (
	"fmt"
	"net/http"

	"github.com/dupakarovsky/eda-storefront/baskets"
	"github.com/nats-io/nats.go"
)

// healthHTTPHandler will ping the postgres connection as well as the NATS jetstream
// to make sure they are online, then return an OK to the caller
func healthHTTPHandler(mod *baskets.Module) http.HandlerFunc {

	fn := func(w http.ResponseWriter, r *http.Request) {

		// try pinging postgres; fails emit an error log and respond to the client
		// with 500 stautus code
		if err := mod.Pool.Ping(r.Context()); err != nil {
			mod.Logger.Error("health: postgres unreachable", "error", err)
			http.Error(w, "database unreachable", http.StatusInternalServerError)
			return
		}

		// very the stauts of the NATS connection; if is not CONNECTED respond
		// to the client with a 500 status
		if mod.NC.Status() != nats.CONNECTED {
			mod.Logger.Error("health: nats not connected", "status", mod.NC.Status().String())
			http.Error(w, "nats not connected", http.StatusInternalServerError)
			return
		}

		// passed here both postgres and nats have responded
		// -------------------------------------------------

		// write the status code
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}

	return http.HandlerFunc(fn)
}
