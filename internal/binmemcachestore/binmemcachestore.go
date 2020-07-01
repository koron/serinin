package binmemcachestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/charithe/mnemosyne/memcache"
	"github.com/koron/serinin/internal/seri"
)

type store struct {
	client    *memcache.Client
	expiresIn time.Duration
	ens       []string
}

var _ seri.Storage = (*store)(nil)

func newStore(cfg *seri.Memcache, ens []string) (*store, error) {
	if cfg == nil {
		return nil, errors.New("\"binmemcache\" configuration is not available")
	}
	if len(cfg.Addrs) == 0 {
		return nil, errors.New("\"addrs\" requires one or more addresses")
	}
	client, err := memcache.NewClient(
		memcache.WithNodePicker(memcache.NewSimpleNodePicker(cfg.Addrs...)),
		memcache.WithConnectionsPerNode(cfg.ConnsPerNode),
	)
	if err != nil {
		return nil, err
	}
	return &store{
		client:    client,
		expiresIn: time.Duration(cfg.ExpireIn),
		ens:       ens,
	}, nil
}

func (mbs *store) StoreRequest(reqid, method, url string) error {
	b, err := json.Marshal(&seri.Response{
		ID:     reqid,
		Method: method,
		URL:    url,
	})
	if err != nil {
		return err
	}
	_, err = mbs.client.Set(context.Background(), []byte(reqid), b, memcache.WithExpiry(mbs.expiresIn))
	return err
}

func (mbs *store) StoreResponse(reqid, name string, data []byte) error {
	_, err := mbs.client.Set(context.Background(), []byte(reqid+"."+name), data, memcache.WithExpiry(mbs.expiresIn))
	return err
}

func (mbs *store) extractName(key string, reqid string) string {
	if !strings.HasPrefix(key, reqid) {
		return ""
	}
	if len(key) <= len(reqid) || key[len(reqid)] != '.' {
		return ""
	}
	return key[len(reqid)+1:]
}

func (mbs *store) GetResponse(reqid string) (*seri.Response, error) {
	keys := make([][]byte, 1, len(mbs.ens))
	keys[0] = []byte(reqid)
	for _, en := range mbs.ens {
		keys = append(keys, []byte(reqid+"."+en))
	}
	rs, err := mbs.client.MultiGet(context.Background(), keys...)
	if err != nil {
		return nil, err
	}

	resp := new(seri.Response)
	resp.Results = make(map[string]string)
	for _, r := range rs {
		if err := r.Err(); err != nil {
			continue
		}
		k := string(r.Key())
		if k == reqid {
			err := json.Unmarshal(r.Value(), resp)
			if err != nil {
				return nil, err
			}
			continue
		}
		if name := mbs.extractName(k, reqid); name != "" {
			resp.Results[name] = string(r.Value())
		}
	}

	return resp, nil
}

func init() {
	seri.RegisterStorage("binmemcache", func(cfg *seri.Config) (seri.Storage, error) {
		return newStore(cfg.BinMemcache, cfg.EntryPointNames())
	})
}
