// customers/cmd/service/health.go
package main

import (
	"fmt"
	"net/http"

	"github.com/dupakarovsky/eda-storefront/customers"
	"github.com/nats-io/nats.go"
)

func healthHTTPHandler(mod *customers.Module) http.HandlerFunc {

	fn := func(w http.ResponseWriter, r *http.Request) {

		if err := mod.Pool.Ping(r.Context()); err != nil {
			mod.Logger.Error("health: postgres unreachable", "error", err)
			http.Error(w, "database unreachable", http.StatusInternalServerError)
			return
		}

		if mod.NC.Status() != nats.CONNECTED {
			mod.Logger.Error("health: nats not connected", "status", mod.NC.Status().String())
			http.Error(w, "nats not connected", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}

	return http.HandlerFunc(fn)
}
