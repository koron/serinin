package seri

import (
	"time"
)

type Config struct {
	Addr string

	ShutdownTimeout time.Duration

	HttpClientTimeout time.Duration

	Endpoints map[string]Endpoint

	Redis Redis
}

func (c *Config) Clone() Config {
	return *c
}

type Endpoint struct {
	URL string
}

type Redis struct {
	Addr     string
	Password string
	DBNum    int
	Expire   time.Duration
}
