package gocachestore

import (
	"errors"
	"fmt"
	"time"

	"github.com/koron/serinin/internal/seri"
	"github.com/patrickmn/go-cache"
)

type storage struct {
	cache     *cache.Cache
	expiresIn time.Duration
	ens       []string
}

var _ seri.Storage = (*storage)(nil)

func newStore(cfg *seri.GoCache, ens []string) (*storage, error) {
	if cfg == nil {
		return nil, errors.New("\"cache\" configuration is not available")
	}
	c := cache.New(time.Duration(cfg.ExpireIn), 15*time.Second)
	return &storage{
		cache:     c,
		expiresIn: time.Duration(cfg.ExpireIn),
		ens:       ens,
	}, nil
}

func (cs *storage) StoreRequest(reqid, method, url string) error {
	cs.cache.SetDefault(reqid, &seri.Response{
		ID:     reqid,
		Method: method,
		URL:    url,
	})
	return nil
}

func (cs *storage) StoreResponse(reqid, name string, data []byte) error {
	cs.cache.SetDefault(reqid+"."+name, data)
	return nil
}

func (cs *storage) GetResponse(reqid string) (*seri.Response, error) {
	v, ok := cs.cache.Get(reqid)
	if !ok {
		return nil, fmt.Errorf("no requests found: %s", reqid)
	}
	resp, ok := v.(*seri.Response)
	if !ok {
		return nil, fmt.Errorf("no requests found: %s", reqid)
	}
	resp.Results = make(map[string]string)
	for _, en := range cs.ens {
		r, ok := cs.cache.Get(reqid + "." + en)
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

func init() {
	seri.RegisterStorage("gocache", func(cfg *seri.Config) (seri.Storage, error) {
		return newStore(cfg.GoCache, cfg.EntryPointNames())
	})
}
