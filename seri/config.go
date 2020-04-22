package seri

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

// Config provides configuration set for `seri.Seri`.
type Config struct {
	Addr string `json:"addr"`

	ShutdownTimeout Duration `json:"shutdown_timeout"`

	// HTTPClientTimeout is default timeout for HTTP client.
	// This will be override by `endpoints["foobar"].timeout`.
	HTTPClientTimeout Duration `json:"http_client_timeout"`

	Endpoints map[string]Endpoint `json:"endpoints"`

	// Redis is redis configuration.
	Redis Redis `json:"redis"`
}

// Clone clones a configuration object.
func (c *Config) Clone() Config {
	return *c
}

// Endpoint is HTTP(S) server to dispatch requests.
type Endpoint struct {
	URL string `json:"url"`

	// Timeout provides timeout duration for each endpoints.
	// default is Config.HTTPClientTimeout, when this is omitted.
	Timeout Duration `json:"timeout,omitempty"`
}

// Redis provides configuration of redis store.
type Redis struct {
	Addr     string   `json:"addr"`
	Password string   `json:"password,omitempty"`
	DBNum    int      `json:"dbnum,omitempty"`
	ExpireIn Duration `json:"expire_in"`
}

// LoadConfig loads a JSON file and parse as `Config`.
func LoadConfig(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c Config
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	err = d.Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Duration provides JSON marshaler/unmarshaler for `time.Duration`.
type Duration time.Duration

// MarshalJSON provides `json.Marshaler`.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON provides `json.Unmarshaler`.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch w := v.(type) {
	case float64:
		*d = Duration(time.Duration(w))
		return nil
	case string:
		x, err := time.ParseDuration(w)
		if err != nil {
			return err
		}
		*d = Duration(x)
		return nil
	default:
		return errors.New("invalid duration")
	}
}
