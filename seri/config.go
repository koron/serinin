package seri

type Config struct {
	Addr string
}

func (c *Config) Clone() Config {
	return *c
}
