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
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/koron-go/ctxsrv"
	"github.com/koron-go/reqlim"
)

// Stat stores statistics for requests.
type Stat struct {
	Inquire        int64
	InquireFail    int64
	InquireTimeout int64
	StoreTimeout   int64
	WorkerFail     int64
}

// Broker traps and dispatch HTTP requests to servers.
// And stores all responses to volatile storage (redis).
type Broker struct {
	cf  Config
	log *log.Logger
	cl  *http.Client

	st Storage

	eps []endpoint
	ens []string

	stat Stat

	worker *Worker
}

type endpoint struct {
	name string
	url  *url.URL
	to   time.Duration
}

func conf2eps(cf *Config) ([]endpoint, error) {
	eps := make([]endpoint, 0, len(cf.Endpoints))
	for n, ep := range cf.Endpoints {
		u, err := url.Parse(ep.URL)
		if err != nil {
			return nil, err
		}
		to := ep.Timeout
		if to <= 0 {
			to = cf.HTTPClientTimeout
		}
		eps = append(eps, endpoint{
			name: n,
			url:  u,
			to:   time.Duration(to),
		})
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
	ens := eps2ens(eps)

	st, err := newStorage(cf, ens)
	if err != nil {
		return nil, err
	}

	var w *Worker
	if cf.WorkerNum > 0 {
		log.Printf("[DEBUG] %d workers launched", cf.WorkerNum)
		w := NewWorker(cf.WorkerNum)
		w.Start()
	}

	b := &Broker{
		cf:     cf.Clone(),
		log:    log.New(os.Stderr, "", log.LstdFlags),
		cl:     newClient(cf),
		st:     st,
		eps:    eps,
		ens:    ens,
		worker: w,
	}
	return b, nil
}

// Close closes broker.
func (b *Broker) Close() {
	if b.worker != nil {
		b.worker.Close()
	}
}

// Serve starts HTTP service.
func (b *Broker) Serve(ctx context.Context) error {
	b.log.Printf("[INFO] broker: listening on %s", b.cf.Addr)
	var h http.Handler = http.HandlerFunc(b.serveHTTP)
	if limit := b.cf.MaxHandlers; limit > 0 {
		b.log.Printf("[DEBUG] max handlers limitation: %d", b.cf.MaxHandlers)
		h = reqlim.Handler(h, limit, "")
	}
	cfg := ctxsrv.HTTP(&http.Server{Addr: b.cf.Addr, Handler: h}).
		WithShutdownTimeout(time.Duration(b.cf.ShutdownTimeout)).
		WithDoneContext(func() {
			b.log.Printf("[INFO] broker: context canceled")
			b.Close()
		}).
		WithDoneServer(func() {
			b.log.Printf("[INFO] broker: closed")
		})
	return cfg.ServeWithContext(ctx)
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
		b.dispatch(w, r, func(reqid string, ep *endpoint, qs string) {
			b.inquire(reqid, ep, "GET", qs, "", nil)
		})

	case "POST":
		d, err := ioutil.ReadAll(r.Body)
		if err != nil {
			b.reportError(w, "(N/A)", 500, "failed to read body", err)
		}
		ct := r.Header.Get("Content-Type")
		b.dispatch(w, r, func(reqid string, ep *endpoint, qs string) {
			b.inquire(reqid, ep, "POST", qs, ct, bytes.NewReader(d))
		})

	default:
		w.Header().Add("Allow", allowMethods)
		b.reportError(w, "", http.StatusMethodNotAllowed, "method not allowed",
			fmt.Errorf("method %s is not allowed", r.Method))
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

func (b *Broker) dispatch(w http.ResponseWriter, r *http.Request, goFn func(reqid string, ep *endpoint, qs string)) {
	reqid, err := b.newReqid()
	if err != nil {
		b.reportError(w, "(N/A)", 500, "failed to genrate ID", err)
		return
	}

	err = b.st.StoreRequest(reqid, r.Method, r.URL.String())
	if err != nil {
		b.reportError(w, reqid, 500, "failed to prepare storage", err)
		return
	}

	qs := r.URL.RawQuery
	if b.worker != nil {
		for i := range b.eps {
			p := &b.eps[i]
			err := b.worker.Run(func() { goFn(reqid, p, qs) })
			if err != nil {
				atomic.AddInt64(&b.stat.WorkerFail, 1)
				b.log.Printf("[WARN] worker: reqid=%s epname=%s: failed to queue: %s", reqid, p.name, err)
			}
		}
	} else {
		go func() {
			for i := range b.eps {
				go goFn(reqid, &b.eps[i], qs)
			}
		}()
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response{
		RequestID: reqid,
		Endpoints: b.ens,
	})
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

var once sync.Once

func (b *Broker) isTimeout(err error) bool {
	var x interface {
		Error() string
		Timeout() bool
	}
	if errors.As(err, &x) && x.Timeout() {
		return true
	}
	return false
}

func (b *Broker) inquire(reqid string, ep *endpoint, method, qs, ct string, body io.Reader) {
	atomic.AddInt64(&b.stat.Inquire, 1)
	ctx := context.Background()
	if ep.to > 0 {
		x, cancel := context.WithTimeout(ctx, ep.to)
		defer cancel()
		ctx = x
	}
	u := b.concatQuery(ep.url, qs).String()
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if err != nil {
		atomic.AddInt64(&b.stat.InquireFail, 1)
		b.log.Printf("[WARN] worker: reqid=%s epname=%s: failed to request: %s", reqid, ep.name, err)
		return
	}
	resp, err := b.cl.Do(req)
	if err != nil {
		if b.isTimeout(err) {
			atomic.AddInt64(&b.stat.InquireTimeout, 1)
			return
		}
		atomic.AddInt64(&b.stat.InquireFail, 1)
		//b.log.Printf("[WARN] worker: reqid=%s epname=%s: failed to round trip: %s", reqid, ep.name, err)
		return
	}
	defer resp.Body.Close()
	//sc := resp.StatusCode
	da, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		atomic.AddInt64(&b.stat.InquireFail, 1)
		b.log.Printf("[WARN] worker: reqid=%s epname=%s: failed to read: %s", reqid, ep.name, err)
		return
	}
	err = b.st.StoreResponse(reqid, ep.name, da)
	if err != nil {
		if b.isTimeout(err) {
			atomic.AddInt64(&b.stat.StoreTimeout, 1)
			return
		}
		atomic.AddInt64(&b.stat.InquireFail, 1)
		b.log.Printf("[WARN] worker: reqid=%s epname=%s: failed to store: %s", reqid, ep.name, err)
		return
	}
}

// Stat gets current Stat, then resets it.
func (b *Broker) Stat() Stat {
	return Stat{
		Inquire:        atomic.SwapInt64(&b.stat.Inquire, 0),
		InquireFail:    atomic.SwapInt64(&b.stat.InquireFail, 0),
		InquireTimeout: atomic.SwapInt64(&b.stat.InquireTimeout, 0),
		StoreTimeout:   atomic.SwapInt64(&b.stat.StoreTimeout, 0),
		WorkerFail:     atomic.SwapInt64(&b.stat.WorkerFail, 0),
	}
}
