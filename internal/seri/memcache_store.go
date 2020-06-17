package seri

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/charithe/mnemosyne/memcache"
)

type memcacheBinStore struct {
	client    *memcache.Client
	expiresIn time.Duration
	ens       []string
}

var _ Storage = (*memcacheBinStore)(nil)

func newMemcacheBinStore(cfg *Memcache, ens []string) (*memcacheBinStore, error) {
	if cfg == nil {
		return nil, errors.New("\"memcache\" is not available")
	}
	if len(cfg.Addrs) == 0 {
		return nil, errors.New("\"addrs\" requires one or more addresses")
	}
	client, err := memcache.NewSimpleClient(cfg.Addrs...)
	if err != nil {
		return nil, err
	}
	return &memcacheBinStore{
		client:    client,
		expiresIn: time.Duration(cfg.ExpireIn),
		ens:       ens,
	}, nil
}

func (mbs *memcacheBinStore) StoreRequest(reqid, method, url string) error {
	b, err := json.Marshal(&memcacheRequestItem{
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

func (mbs *memcacheBinStore) StoreResponse(reqid, name string, data []byte) error {
	_, err := mbs.client.Set(context.Background(), []byte(reqid+"."+name), data, memcache.WithExpiry(mbs.expiresIn))
	return err
}

func (mbs *memcacheBinStore) extractName(key string, reqid string) string {
	if !strings.HasPrefix(key, reqid) {
		return ""
	}
	if len(key) <= len(reqid) || key[len(reqid)] != '.' {
		return ""
	}
	return key[len(reqid)+1:]
}

func (mbs *memcacheBinStore) GetResponse(reqid string) (*Response, error) {
	keys := make([][]byte, 1, len(mbs.ens))
	keys[0] = []byte(reqid)
	for _, en := range mbs.ens {
		keys = append(keys, []byte(reqid+"."+en))
	}
	rs, err := mbs.client.MultiGet(context.Background(), keys...)
	if err != nil {
		return nil, err
	}

	resp := new(Response)
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
