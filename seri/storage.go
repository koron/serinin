package seri

import (
	"log"
	"time"

	"github.com/go-redis/redis"
)

// Storage is requirements to store results.
type Storage interface {
	Store(reqid, method, url string, results []*Result) error
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

func (rs *redisStore) Store(reqid, method, url string, results []*Result) error {
	d := make(map[string]interface{}, len(results)+3)
	d["_id"] = reqid
	d["_method"] = method
	d["_url"] = url
	for _, r := range results {
		if len(r.Data) > 0 {
			d[r.Name] = r.Data
		}
	}
	p := rs.client.TxPipeline()
	p.HMSet(reqid, d)
	if rs.expiresIn > 0 {
		p.Expire(reqid, time.Duration(rs.expiresIn)).Result()
	}
	_, err := p.Exec()
	if err != nil {
		return err
	}
	return nil
}

var _ Storage = (*redisStore)(nil)

type discardStore struct{}

func (*discardStore) Store(reqid, method, url string, results []*Result) error {
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
