package seri

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-redis/redis"
)

// Storage is requirements to store results.
type Storage interface {
	StoreRequest(reqid, method, url string) error

	StoreResponse(reqid, name string, data []byte) error
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

var _ Storage = (*redisStore)(nil)

type discardStore struct{}

func (*discardStore) StoreRequest(reqid, method, url string) error {
	return nil
}

func (*discardStore) StoreResponse(reqid, name string, data []byte) error {
	return nil
}

var _ Storage = (*discardStore)(nil)

type memcacheStore struct {
	client    *memcache.Client
	expiresIn int32
}

var _ Storage = (*memcacheStore)(nil)

func newMemcacheStore(cfg *Memcache) (*memcacheStore, error) {
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

func newStorage(cf *Config) (Storage, error) {
	switch cf.StoreType {
	case "", "discard":
		return &discardStore{}, nil
	case "redis":
		return newRedisStore(cf.Redis)
	case "memcache":
		return newMemcacheStore(cf.Memcache)
	default:
		return nil, fmt.Errorf("unsupported \"store_type\": %q", cf.StoreType)
	}
}
