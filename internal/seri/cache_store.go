package seri

import (
	"errors"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
)

type cacheStore struct {
	cache     *cache.Cache
	expiresIn time.Duration
	ens       []string
}

var _ Storage = (*cacheStore)(nil)

func newCacheStore(cfg *Cache, ens []string) (*cacheStore, error) {
	if cfg == nil {
		return nil, errors.New("\"cache\" is not available")
	}
	c := cache.New(time.Duration(cfg.ExpireIn), 15*time.Second)
	return &cacheStore{
		cache:     c,
		expiresIn: time.Duration(cfg.ExpireIn),
		ens:       ens,
	}, nil
}

func (cs *cacheStore) StoreRequest(reqid, method, url string) error {
	cs.cache.SetDefault(reqid, &memcacheRequestItem{
		ID:     reqid,
		Method: method,
		URL:    url,
	})
	return nil
}

func (cs *cacheStore) StoreResponse(reqid, name string, data []byte) error {
	cs.cache.SetDefault(reqid+"."+name, data)
	return nil
}

func (cs *cacheStore) GetResponse(reqid string) (*Response, error) {
	v, ok := cs.cache.Get(reqid)
	if !ok {
		return nil, fmt.Errorf("no requests found: %s", reqid)
	}
	x, ok := v.(*memcacheRequestItem)
	if !ok {
		return nil, fmt.Errorf("no requests found: %s", reqid)
	}

	resp := &Response{
		ID:      x.ID,
		Method:  x.Method,
		URL:     x.URL,
		Results: make(map[string]string),
	}

	for _, en := range cs.ens {
		r, ok := cs.cache.Get(reqid+"."+en)
		if !ok {
			continue
		}
		b, ok := r.([]byte)
		if !ok {
			continue
		}
		var v string
		if len(b) > 0 {
			v = string(b)
		}
		resp.Results[en] = v
	}

	return resp, nil
}
