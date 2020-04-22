package main

// Config is configuration for program.
type Config struct {
	StartPort int
	Count     int
}

// Clone creates a copy of configuration.
func (cf *Config) Clone() Config {
	c := *cf
	return c
}
