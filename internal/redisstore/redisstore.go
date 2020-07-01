package redisstore

import (
	"errors"
	"strings"
	"time"

	redis "github.com/go-redis/redis/v7"
	"github.com/koron/serinin/internal/seri"
)

type storage struct {
	client    *redis.Client
	expiresIn seri.Duration
}

var _ seri.Storage = (*storage)(nil)

func newStorage(cfg *seri.Redis) (*storage, error) {
	if cfg == nil {
		return nil, errors.New("\"redis\" is not available")
	}
	return &storage{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DBNum,
			PoolSize: cfg.PoolSize,
		}),
		expiresIn: cfg.ExpireIn,
	}, nil
}

func (rs *storage) StoreRequest(reqid, method, url string) error {
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

func (rs *storage) StoreResponse(reqid, name string, data []byte) error {
	_, err := rs.client.HSet(reqid, name, data).Result()
	if err != nil {
		return err
	}
	return nil
}

func (rs *storage) GetResponse(reqid string) (*seri.Response, error) {
	m, err := rs.client.HGetAll(reqid).Result()
	if err != nil {
		return nil, err
	}
	r := &seri.Response{
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
	return r, nil
}

func init() {
	seri.RegisterStorage("redis", func(cfg *seri.Config) (seri.Storage, error) {
		return newStorage(cfg.Redis)
	})
}
