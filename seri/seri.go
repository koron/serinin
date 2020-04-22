package seri

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/koron-go/ctxsrv"
)

// Broker traps and dispatch HTTP requests to servers.
// And stores all responses to volatile storage (redis).
type Broker struct {
	cf    Config
	log   *log.Logger
	cl    *http.Client
	redis *redis.Client

	eps []endpoint
	ens []string
}

type endpoint struct {
	name string
	url  *url.URL
}

func conf2eps(cf *Config) ([]endpoint, error) {
	eps := make([]endpoint, 0, len(cf.Endpoints))
	for n, ep := range cf.Endpoints {
		u, err := url.Parse(ep.URL)
		if err != nil {
			return nil, err
		}
		eps = append(eps, endpoint{name: n, url: u})
	}
	return eps, nil
}

func eps2ens(eps []endpoint) []string {
	ens := make([]string, len(eps))
	for i, ep := range eps {
		ens[i] = ep.name
	}
	return ens
}

// NewBroker creates a new `Broker`
func NewBroker(cf *Config) (*Broker, error) {
	if len(cf.Endpoints) == 0 {
		return nil, errors.New("no endpoints")
	}
	eps, err := conf2eps(cf)
	if err != nil {
		return nil, err
	}
	b := &Broker{
		cf:  cf.Clone(),
		log: log.New(os.Stdout, "", log.LstdFlags),
		cl: &http.Client{
			Timeout: time.Duration(cf.HTTPClientTimeout),
		},
		redis: redis.NewClient(&redis.Options{
			Addr:     cf.Redis.Addr,
			Password: cf.Redis.Password,
			DB:       cf.Redis.DBNum,
		}),
		eps: eps,
		ens: eps2ens(eps),
	}
	return b, nil
}

// Serve starts HTTP service.
func (b *Broker) Serve(ctx context.Context) error {
	b.log.Printf("server: listening on %b", b.cf.Addr)
	return ctxsrv.HTTP(&http.Server{
		Addr:    b.cf.Addr,
		Handler: http.HandlerFunc(b.serveHTTP),
	}).WithShutdownTimeout(time.Duration(b.cf.ShutdownTimeout)).
		WithDoneContext(func() {
			b.log.Printf("server: context canceled")
		}).
		WithDoneServer(func() {
			b.log.Printf("server: closed")
		}).
		ServeWithContext(ctx)
}

func (b *Broker) reportError(w http.ResponseWriter, reqid string, code int, title string, err error) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(&problemDetail{
		Status:    code,
		Title:     title,
		Detail:    err.Error(),
		RequestID: reqid,
	})
}

var allowMethods = "GET, POST"

func (b *Broker) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		b.seriGet(w, r)
	case "POST":
		b.seriPost(w, r)
	default:
		w.Header().Add("Allow", allowMethods)
		b.reportError(w, "", http.StatusMethodNotAllowed, "method not allowed",
			fmt.Errorf("method %s is not allowed"))
	}
}

func (b *Broker) newReqid() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

type problemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`

	RequestID string `json:"request_id,omitempty"`
}

type response struct {
	RequestID string   `json:"request_id"`
	Endpoints []string `json:"endpoints"`
}

func (b *Broker) seriGet(w http.ResponseWriter, r *http.Request) {
	reqid, err := b.newReqid()
	if err != nil {
		b.reportError(w, "(N/A)", 500, "failed to genrate ID", err)
		return
	}
	err = b.storeRequest(reqid, r)
	if err != nil {
		b.reportError(w, reqid, 500, "failed to prepare storage", err)
		return
	}
	b.log.Printf("worker: reqid=%s method=%s: accept", reqid, r.Method)
	for _, ep := range b.eps {
		go b.goGet(reqid, ep.name, ep.url.String())
	}
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response{
		RequestID: reqid,
		Endpoints: b.ens,
	})
}

func (b *Broker) goGet(reqid, epname, url string) {
	resp, err := b.cl.Get(url)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to request: %s", reqid, epname, err)
		return
	}
	defer resp.Body.Close()
	sc := resp.StatusCode
	da, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to read: %s", reqid, epname, err)
		return
	}
	err = b.storeResponse(reqid, epname, sc, da)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to store: %s", reqid, epname, err)
		return
	}
	b.log.Printf("worker: reqid=%s epname=%s: success", reqid, epname)
}

func (b *Broker) seriPost(w http.ResponseWriter, r *http.Request) {
	reqid, err := b.newReqid()
	if err != nil {
		b.reportError(w, "(N/A)", 500, "failed to genrate ID", err)
		return
	}
	d, err := ioutil.ReadAll(r.Body)
	if err != nil {
		b.reportError(w, reqid, 500, "failed to read body", err)
	}
	err = b.storeRequest(reqid, r)
	if err != nil {
		b.reportError(w, reqid, 500, "failed to prepare storage", err)
		return
	}
	b.log.Printf("worker: reqid=%s method=%s: accept", reqid, r.Method)
	ct := r.Header.Get("Content-Type")
	for _, ep := range b.eps {
		go b.goPost(reqid, ep.name, ep.url.String(), ct, bytes.NewReader(d))
	}
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response{
		RequestID: reqid,
		Endpoints: b.ens,
	})
}

func (b *Broker) goPost(reqid, epname, url, contentType string, body io.Reader) {
	resp, err := b.cl.Post(url, contentType, body)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to request: %s", reqid, epname, err)
		return
	}
	defer resp.Body.Close()
	sc := resp.StatusCode
	da, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to read: %s", reqid, epname, err)
		return
	}
	err = b.storeResponse(reqid, epname, sc, da)
	if err != nil {
		b.log.Printf("worker: reqid=%s epname=%s: failed to store: %s", reqid, epname, err)
		return
	}
	b.log.Printf("worker: reqid=%s epname=%s: success", reqid, epname)
}

func (b *Broker) concatQuery(base *url.URL, q string) *url.URL {
	if q == "" {
		return base
	}
	u := *base
	if u.RawQuery != "" {
		u.RawQuery += "&" + q
	} else {
		u.RawQuery = q
	}
	return &u
}

func (b *Broker) storeRequest(reqid string, r *http.Request) error {
	_, err := b.redis.HMSet(reqid, map[string]interface{}{
		"_id":     reqid,
		"_method": r.Method,
		"_url":    r.URL.String(),
	}).Result()
	if err != nil {
		return err
	}
	if b.cf.Redis.ExpireIn <= 0 {
		return nil
	}
	_, err = b.redis.Expire(reqid, time.Duration(b.cf.Redis.ExpireIn)).Result()
	if err != nil {
		return err
	}
	return nil
}

func (b *Broker) storeResponse(reqid, epname string, statusCode int, data []byte) error {
	b.log.Printf("store: reqid=%s epname=%s sc=%d: stored", reqid, epname, statusCode)
	_, err := b.redis.HSet(reqid, epname, data).Result()
	if err != nil {
		return err
	}
	return nil
}
