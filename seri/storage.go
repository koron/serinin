package seri

import (
	"log"
	"time"

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

func newRedisStore(cfg *Redis) *redisStore {
	return &redisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DBNum,
		}),
		expiresIn: cfg.ExpireIn,
	}
}

func (rs *redisStore) StoreRequest(reqid, method, url string) error {
	_, err := rs.client.HMSet(reqid, map[string]interface{}{
		"_id":     reqid,
		"_method": method,
		"_url":    url,
	}).Result()
	if err != nil {
		return err
	}
	if rs.expiresIn <= 0 {
		return nil
	}
	_, err = rs.client.Expire(reqid, time.Duration(rs.expiresIn)).Result()
	if err != nil {
		return err
	}
	return nil
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

func newStorage(cf *Config) Storage {
	if cf.Redis != nil {
		return newRedisStore(cf.Redis)
	}
	log.Printf("[WARN] no redis setting, will discard all responses")
	return &discardStore{}
}
