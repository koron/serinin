package seri

import (
	"net"
	"net/http"
	"time"
)

func newClient(cf *Config) *http.Client {
	n := cf.MaxHandlers
	m := n * len(cf.Endpoints)
	if m == 0 {
		m = 100
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          m,
			MaxIdleConnsPerHost:   n,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 20 * time.Second,
	}
}
