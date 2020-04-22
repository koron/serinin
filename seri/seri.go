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

type Seri struct {
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

func New(cf *Config) (*Seri, error) {
	if len(cf.Endpoints) == 0 {
		return nil, errors.New("no endpoints")
	}
	eps, err := conf2eps(cf)
	if err != nil {
		return nil, err
	}
	s := &Seri{
		cf:  cf.Clone(),
		log: log.New(os.Stdout, "", log.LstdFlags),
		cl: &http.Client{
			Timeout: time.Duration(cf.HttpClientTimeout),
		},
		redis: redis.NewClient(&redis.Options{
			Addr:     cf.Redis.Addr,
			Password: cf.Redis.Password,
			DB:       cf.Redis.DBNum,
		}),
		eps: eps,
		ens: eps2ens(eps),
	}
	return s, nil
}

func (s *Seri) Serve(ctx context.Context) error {
	s.log.Printf("server: listening on %s", s.cf.Addr)
	return ctxsrv.HTTP(&http.Server{
		Addr:    s.cf.Addr,
		Handler: http.HandlerFunc(s.serveHTTP),
	}).WithShutdownTimeout(time.Duration(s.cf.ShutdownTimeout)).
		WithDoneContext(func() {
			s.log.Printf("server: context canceled")
		}).
		WithDoneServer(func() {
			s.log.Printf("server: closed")
		}).
		ServeWithContext(ctx)
}

func (s *Seri) reportError(w http.ResponseWriter, reqid string, code int, title string, err error) {
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

func (s *Seri) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.seriGet(w, r)
	case "POST":
		s.seriPost(w, r)
	default:
		w.Header().Add("Allow", allowMethods)
		s.reportError(w, "", http.StatusMethodNotAllowed, "method not allowed",
			fmt.Errorf("method %s is not allowed"))
	}
}

func (s *Seri) newReqid() (string, error) {
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

func (s *Seri) seriGet(w http.ResponseWriter, r *http.Request) {
	reqid, err := s.newReqid()
	if err != nil {
		s.reportError(w, "(N/A)", 500, "failed to genrate ID", err)
		return
	}
	err = s.storeRequest(reqid, r)
	if err != nil {
		s.reportError(w, reqid, 500, "failed to prepare storage", err)
		return
	}
	s.log.Printf("worker: reqid=%s method=%s: accept", reqid, r.Method)
	for _, ep := range s.eps {
		go s.goGet(reqid, ep.name, ep.url.String())
	}
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response{
		RequestID: reqid,
		Endpoints: s.ens,
	})
}

func (s *Seri) goGet(reqid, epname, url string) {
	resp, err := s.cl.Get(url)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to request: %s", reqid, epname, err)
		return
	}
	defer resp.Body.Close()
	sc := resp.StatusCode
	da, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to read: %s", reqid, epname, err)
		return
	}
	err = s.storeResponse(reqid, epname, sc, da)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to store: %s", reqid, epname, err)
		return
	}
	s.log.Printf("worker: reqid=%s epname=%s: success", reqid, epname)
}

func (s *Seri) seriPost(w http.ResponseWriter, r *http.Request) {
	reqid, err := s.newReqid()
	if err != nil {
		s.reportError(w, "(N/A)", 500, "failed to genrate ID", err)
		return
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.reportError(w, reqid, 500, "failed to read body", err)
	}
	err = s.storeRequest(reqid, r)
	if err != nil {
		s.reportError(w, reqid, 500, "failed to prepare storage", err)
		return
	}
	s.log.Printf("worker: reqid=%s method=%s: accept", reqid, r.Method)
	ct := r.Header.Get("Content-Type")
	for _, ep := range s.eps {
		go s.goPost(reqid, ep.name, ep.url.String(), ct, bytes.NewReader(b))
	}
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response{
		RequestID: reqid,
		Endpoints: s.ens,
	})
}

func (s *Seri) goPost(reqid, epname, url, contentType string, body io.Reader) {
	resp, err := s.cl.Post(url, contentType, body)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to request: %s", reqid, epname, err)
		return
	}
	defer resp.Body.Close()
	sc := resp.StatusCode
	da, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to read: %s", reqid, epname, err)
		return
	}
	err = s.storeResponse(reqid, epname, sc, da)
	if err != nil {
		s.log.Printf("worker: reqid=%s epname=%s: failed to store: %s", reqid, epname, err)
		return
	}
	s.log.Printf("worker: reqid=%s epname=%s: success", reqid, epname)
}

func (s *Seri) concatQuery(base *url.URL, q string) *url.URL {
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

func (s *Seri) storeRequest(reqid string, r *http.Request) error {
	_, err := s.redis.HMSet(reqid, map[string]interface{}{
		"_id":     reqid,
		"_method": r.Method,
		"_url":    r.URL.String(),
	}).Result()
	if err != nil {
		return err
	}
	if s.cf.Redis.ExpireIn <= 0 {
		return nil
	}
	_, err = s.redis.Expire(reqid, time.Duration(s.cf.Redis.ExpireIn)).Result()
	if err != nil {
		return err
	}
	return nil
}

func (s *Seri) storeResponse(reqid, epname string, statusCode int, data []byte) error {
	s.log.Printf("store: reqid=%s epname=%s sc=%d: stored", reqid, epname, statusCode)
	_, err := s.redis.HSet(reqid, epname, data).Result()
	if err != nil {
		return err
	}
	return nil
}
