package seri

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"
)

// Config provides configuration set for `seri.Seri`.
type Config struct {
	Addr string `json:"addr"`

	ShutdownTimeout Duration `json:"shutdown_timeout"`

	// MaxHandlers limits the number of http.Handler ServeHTTP goroutines
	// which may run at a time over all connections.
	// Negative or zero no limit.
	MaxHandlers int `json:"max_handlers"`

	// WorkerNum declare number of workers.
	WorkerNum int `json:"worker_num"`

	// HTTPClientTimeout is default timeout for HTTP client.
	// This will be override by `endpoints["foobar"].timeout`.
	HTTPClientTimeout Duration `json:"http_client_timeout"`

	Endpoints map[string]Endpoint `json:"endpoints"`

	// StoreType specify store type: "none", "redis", "memcache",
	// "binmemcache", "gocache"
	StoreType string `json:"store_type"`

	// Redis is redis configuration.
	Redis *Redis `json:"redis,omitempty"`

	// Memcache is memcache configuration.
	Memcache *Memcache `json:"memcache,omitempty"`

	// BinMemcache is memcache configuration with binary protocol.
	BinMemcache *Memcache `json:"binmemcache,omitempty"`

	// GoCache is configuration for in-process memory cache.
	GoCache *GoCache `json:"gocache,omitempty"`
}

// Clone clones a configuration object.
func (c *Config) Clone() Config {
	return *c
}

func (c *Config) EntryPointNames() []string {
	names := make([]string, 0, len(c.Endpoints))
	for n := range c.Endpoints {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
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

	// PoolSize is for size of connection pool.
	// Default zero means 10 times of CPU number (runtime.NumCPU()).
	PoolSize int `json:"pool_size"`
}

// Memcache provides configuration of memcache store.
type Memcache struct {
	Addrs    []string `json:"addrs"`
	ExpireIn Duration `json:"expire_in"`

	// MaxIdleConns limitates number of idle connections. This is available for
	// "memcache" store only.
	MaxIdleConns int `json:"max_idle_conns"`
	// ConnsPerNode limitates number of connections for a node. This is
	// available for "binmemcache" store only.
	ConnsPerNode int `json:"conns_per_node"`
}

// GoCache provides configuration of go-cache store.
type GoCache struct {
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
