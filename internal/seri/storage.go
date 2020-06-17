package seri

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-redis/redis"
)

type Response struct {
	ID      string            `json:"_id"`
	Method  string            `json:"_method"`
	URL     string            `json:"_url"`
	Results map[string]string `json:"results"`
}

// Storage is requirements to store results.
type Storage interface {
	StoreRequest(reqid, method, url string) error

	StoreResponse(reqid, name string, data []byte) error

	GetResponse(reqid string) (*Response, error)
}

type redisStore struct {
	client    *redis.Client
	expiresIn Duration
}

func newRedisStore(cfg *Redis) (*redisStore, error) {
	if cfg == nil {
		return nil, errors.New("\"redis\" is not available")
	}
	return &redisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DBNum,
		}),
		expiresIn: cfg.ExpireIn,
	}, nil
}

func (rs *redisStore) StoreRequest(reqid, method, url string) error {
	p := rs.client.TxPipeline()
	p.HMSet(reqid, map[string]interface{}{
		"_id":     reqid,
		"_method": method,
		"_url":    url,
	}).Result()
	if rs.expiresIn > 0 {
		p.Expire(reqid, time.Duration(rs.expiresIn)).Result()
	}
	_, err := p.Exec()
	return err
}

func (rs *redisStore) StoreResponse(reqid, name string, data []byte) error {
	_, err := rs.client.HSet(reqid, name, data).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rs *redisStore) GetResponse(reqid string) (*Response, error) {
	m, err := rs.client.HGetAll(reqid).Result()
	if err != nil {
		return nil, err
	}
	r := &Response{
		ID:      m["_id"],
		Method:  m["_method"],
		URL:     m["_url"],
		Results: make(map[string]string),
	}
	for k, v := range m {
		if strings.HasPrefix(k, "_") {
			// FIXME: use dictionary to reserved keywords.
			continue
		}
		r.Results[k] = v
	}
	return nil, nil
}

var _ Storage = (*redisStore)(nil)

type discardStore struct{}

func (*discardStore) StoreRequest(reqid, method, url string) error {
	return nil
}

func (*discardStore) StoreResponse(reqid, name string, data []byte) error {
	return nil
}

func (*discardStore) GetResponse(reqid string) (*Response, error) {
	return &Response{ID: reqid}, nil
}

var _ Storage = (*discardStore)(nil)

type memcacheStore struct {
	client    *memcache.Client
	expiresIn int32
	ens       []string
}

var _ Storage = (*memcacheStore)(nil)

func newMemcacheStore(cfg *Memcache, ens []string) (*memcacheStore, error) {
	if cfg == nil {
		return nil, errors.New("\"memcache\" is not available")
	}
	if len(cfg.Addrs) == 0 {
		return nil, errors.New("\"addrs\" requires one or more addresses")
	}
	if time.Duration(cfg.ExpireIn) < time.Second {
		return nil, fmt.Errorf("\"expire_in\" must be larger than 1 second: %s", cfg.ExpireIn)
	}
	return &memcacheStore{
		client:    memcache.New(cfg.Addrs...),
		expiresIn: int32(time.Duration(cfg.ExpireIn) / time.Second),
		ens:       ens,
	}, nil
}

type memcacheRequestItem struct {
	ID     string `json:"_id"`
	Method string `json:"_method"`
	URL    string `json:"_url"`
}

func (ms *memcacheStore) StoreRequest(reqid, method, url string) error {
	b, err := json.Marshal(&memcacheRequestItem{
		ID:     reqid,
		Method: method,
		URL:    url,
	})
	if err != nil {
		return err
	}
	return ms.client.Set(&memcache.Item{
		Key:        reqid,
		Value:      b,
		Expiration: ms.expiresIn,
	})
}

func (ms *memcacheStore) StoreResponse(reqid, name string, data []byte) error {
	return ms.client.Set(&memcache.Item{
		Key:        reqid + "." + name,
		Value:      data,
		Expiration: ms.expiresIn,
	})
}

func (ms *memcacheStore) GetResponse(reqid string) (*Response, error) {
	keys := make([]string, 1, len(ms.ens))
	keys[0] = reqid
	for _, en := range ms.ens {
		keys = append(keys, reqid+"."+en)
	}
	rs, err := ms.client.GetMulti(keys)
	if err != nil {
		return nil, err
	}

	r0, ok := rs[reqid]
	if !ok {
		return nil, fmt.Errorf("no requests found: %s", reqid)
	}
	resp := new(Response)
	err = json.Unmarshal(r0.Value, resp)
	if err != nil {
		return nil, err
	}

	resp.Results = make(map[string]string)
	for _, en := range ms.ens {
		r, ok := rs[reqid+"."+en]
		if !ok {
			continue
		}
		var v string
		if len(r.Value) > 0 {
			v = string(r.Value)
		}
		resp.Results[en] = v
	}

	return resp, nil
}

func newStorage(cf *Config, ens []string) (Storage, error) {
	switch cf.StoreType {
	case "", "discard":
		return &discardStore{}, nil
	case "redis":
		return newRedisStore(cf.Redis)
	case "memcache":
		return newMemcacheStore(cf.Memcache, ens)
	case "memcache-bin":
		return newMemcacheBinStore(cf.Memcache, ens)
	default:
		return nil, fmt.Errorf("unsupported \"store_type\": %q", cf.StoreType)
	}
}

// NewStorage creates a storage by configuration.
func NewStorage(cf *Config) (Storage, error) {
	eps, err := conf2eps(cf)
	if err != nil {
		return nil, err
	}
	ens := eps2ens(eps)
	return newStorage(cf, ens)
}
