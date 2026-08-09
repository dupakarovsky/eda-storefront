package shutdown

import (
	"log/slog"
	"net/http"
	"time"
)

func Run(logger *slog.Logger, srv *http.Server, shutdownTimeout time.Duration) error {
	return nil
}
