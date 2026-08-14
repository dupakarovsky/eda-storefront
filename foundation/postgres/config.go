package postgres

import (
	"time"
)

type SSLMode string

const SSLModeDisabled SSLMode = "disable"
const SSLModeRequire SSLMode = "require"

func (s SSLMode) String() string {
	switch s {
	case SSLModeDisabled:
		return "disable"
	case SSLModeRequire:
		return "require"
	default:
		return "unkown mode"
	}
}

// Config holds every value NewPool needs to build and tune a
// pgxpool.Pool. Deliberately its own type in this package rather than
// folded into cmd/api's config — this package knows nothing about CLI
// flags or env vars, main.go knows nothing about pgxpool internals; main
// builds one of these and hands it in, same dependency direction as
// passing a *slog.Logger into NewCreateReservation.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Schema   string  // "cr" - used to set search_path so unqualified queries can resolve correctly
	SSLMode  SSLMode // e.g "disable" | "require"

	// Pool tuning knobs. All left at Go's zero value by default — NewPool
	// only overrides pgxpool's own built-in defaults when a caller
	// actually supplies a non-zero value here (see NewPool's comment for
	// why that distinction matters).
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

//
//....
//
