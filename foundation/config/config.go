package config

import "time"

// Config holds the three distincts configuration structs: one for
// the server, one for the logger and one for the database
type Config struct {
	Server ServerConfig
	Logger LoggerConfig
	DB     DBConfig
}

// ServerConfig defines the configuration parameters for the server
// using the envcofig package
type ServerConfig struct {
	Host              string        `envconfig:"SRV_HOST" default:""`
	Port              string        `envconfig:"SRV_PORT" default:"8080"`
	ReadHeaderTimeout time.Duration `envconfig:"SRV_READ_HEADER_TIMEOUT" default:"5s"`
	WriteTimeout      time.Duration `envconfig:"SRV_WRITE_TIMEOUT" default:"10s"`
	IdleTimeout       time.Duration `envconfig:"SRV_IDLE_TIMEOUT" default:"120s"`
	ShutdownTimeout   time.Duration `envconfig:"SRV_SHUTDOWN_TIMEOUT" default:"10s"`
	RequestTimeout    time.Duration `envconfig:"SRV_REQUEST_TIMEOUT" default:"3s"`
}

// Logger config defines configuration params for the log's level in the terminal
// and in the .log file
type LoggerConfig struct {
	TermLevel string `envconfig:"LOG_TERM_LEVEL" default:"debug"`
	FileLevel string `envconfig:"LOG_FILE_LEVEL" default:"info"`
}

// DBConfig defines configuration paramters for the postgres database
type DBConfig struct {
	Host            string        `envconfig:"DB_HOST" default:"localhost"`
	Port            int           `envconfig:"DB_PORT" default:"5432"`
	User            string        `envconfig:"DB_USER"`
	Password        string        `envconfig:"DB_PASSWORD"`
	Database        string        `envconfig:"DB_DATABASE" default:"test"`
	Schema          string        `envconfig:"DB_SCHEMA"`
	SSLMode         string        `envconfig:"DB_SSL_MODE" default:"require"`
	MaxConns        int           `envconfig:"DB_MAX_CONNS" default:"15"`
	MinConns        int           `envconfig:"DB_MIN_CONNS" default:"5"`
	MaxConnLifetime time.Duration `envconfig:"DB_MAX_CONNS_LIFETIME" default:"1h"`
	MaxConnIdleTime time.Duration `envconfig:"DB_MAX_CONNS_IDLE_TIME" default:"30m"`
}
