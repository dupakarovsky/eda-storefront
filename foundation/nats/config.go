package foundationnats

import "time"

// Config sets the configuration params for the NATS server
type Config struct {
	URL            string
	Name           string
	ConnectTimeout time.Duration
	MaxReconnects  int
	ReconnectWait  time.Duration
}
