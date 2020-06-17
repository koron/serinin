package memcachestore

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/koron/serinin/internal/seri"
)

type store struct {
	client    *memcache.Client
	expiresIn int32
	ens       []string
}

var _ seri.Storage = (*store)(nil)

func newStore(cfg *seri.Memcache, ens []string) (*store, error) {
	if cfg == nil {
		return nil, errors.New("\"memcache\" is not available")
	}
	if len(cfg.Addrs) == 0 {
		return nil, errors.New("\"addrs\" requires one or more addresses")
	}
	if time.Duration(cfg.ExpireIn) < time.Second {
		return nil, fmt.Errorf("\"expire_in\" must be larger than 1 second: %s", cfg.ExpireIn)
	}
	c := memcache.New(cfg.Addrs...)
	if cfg.MaxIdleConns > 0 {
		c.MaxIdleConns = cfg.MaxIdleConns
	}
	return &store{
		client:    c,
		expiresIn: int32(time.Duration(cfg.ExpireIn) / time.Second),
		ens:       ens,
	}, nil
}

func (ms *store) StoreRequest(reqid, method, url string) error {
	b, err := json.Marshal(&seri.Response{
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

func (ms *store) StoreResponse(reqid, name string, data []byte) error {
	return ms.client.Set(&memcache.Item{
		Key:        reqid + "." + name,
		Value:      data,
		Expiration: ms.expiresIn,
	})
}

func (ms *store) GetResponse(reqid string) (*seri.Response, error) {
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
	resp := new(seri.Response)
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

func init() {
	seri.RegisterStorage("memcache", func(cfg *seri.Config) (seri.Storage, error) {
		return newStore(cfg.Memcache, cfg.EntryPointNames())
	})
}
