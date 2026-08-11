// foundation/shutdown/shutdown.go
package shutdown

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run will execute the graceful shutdown to each service server.
func Run(logger *slog.Logger, srv *http.Server, shutdownTimeout time.Duration) error {

	sig := make(chan os.Signal, 1)
	errs := make(chan error, 1)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("🚀 starting server",
			"srv-addr", srv.Addr,
			"srv-read-header-timeout", srv.ReadHeaderTimeout,
			"srv-write-timeout", srv.WriteTimeout,
			"srv-idle-timeout", srv.IdleTimeout,
			"srv-shutdown-timeout", shutdownTimeout,
		)
		errs <- srv.ListenAndServe()
		logger.Info("goroutine terminated")
	}()

	select {
	case err := <-errs:
		logger.Error("ListenAndServe error", "error", err)
		return err

	case s := <-sig:
		logger.Warn("shutdown signal received. beginning shutdown", "signal", s)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server failed to shutdown", "error", err)
		srv.Close()
		return fmt.Errorf("shutdown error: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}
