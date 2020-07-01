package seri

import (
	"fmt"
)

// Response provides response's information which include request information
// and responses from each end points.
type Response struct {
	ID      string            `json:"_id"`
	Method  string            `json:"_method"`
	URL     string            `json:"_url"`
	Results map[string]string `json:"results,omitempty"`
}

// Storage is requirements to store results.
type Storage interface {
	StoreRequest(reqid, method, url string) error

	StoreResponse(reqid, name string, data []byte) error

	GetResponse(reqid string) (*Response, error)
}

// StorageFactoryFunc is function to create storage implementation.
type StorageFactoryFunc func(*Config) (Storage, error)

var storages = map[string]StorageFactoryFunc{}

// RegisterStorage registers a factory function of Storage
func RegisterStorage(name string, fn StorageFactoryFunc) {
	storages[name] = fn
}

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

func newStorage(cf *Config, ens []string) (Storage, error) {
	if fa, ok := storages[cf.StoreType]; ok {
		return fa(cf)
	}
	switch cf.StoreType {
	case "", "discard":
		return &discardStore{}, nil
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
