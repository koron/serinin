package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Server provides test server for serinin Broker
type Server struct {
	cf  Config
	log *log.Logger
}

// NewServer creates new `Server`
func NewServer(cf *Config) (*Server, error) {
	return &Server{
		cf:  cf.Clone(),
		log: log.New(os.Stdout, "", log.LstdFlags),
	}, nil
}

// ServeAll starts all HTTP servers.
func (s *Server) ServeAll(ctx context.Context) error {
	if s.cf.Count <= 0 {
		return fmt.Errorf("no servers to start: %d", s.cf.Count)
	}
	var wg sync.WaitGroup
	for i := 0; i < s.cf.Count; i++ {
		wg.Add(1)
		go func(n int) {
			s.serve(ctx, n, s.cf.StartPort+n)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return nil
}

func (s *Server) serve(ctx context.Context, id, port int) error {
	serial := 0
	var mu sync.Mutex
	getSerial := func() int {
		mu.Lock()
		defer mu.Unlock()
		serial++
		return serial
	}
	sleepKey := fmt.Sprintf("sleep.%d", id)
	srv := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.log.Printf("dstsrv#%d: receive %s", id, r.URL.String())
			if x := r.URL.Query().Get(sleepKey); x != "" {
				d, err := time.ParseDuration(x)
				if err != nil {
					s.log.Printf("dstsrv#%d: invalid sleep: %s", id, err)
				} else {
					time.Sleep(d)
				}
			}

			w.Header().Add("Context-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "dst_id=%d serial=%d\n", id, getSerial())
		}),
	}
	ctxSrv, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	ch := make(chan error)
	go func() {
		select {
		case <-ctx.Done():
			s.log.Printf("dstsrv#%d: context canceled", id)
			ch <- srv.Shutdown(context.Background())
		case <-ctxSrv.Done():
			s.log.Printf("dstsrv#%d: closed", id)
			ch <- nil
		}
		close(ch)
	}()
	s.log.Printf("dstsrv#%d: running on %s", id, srv.Addr)
	err := srv.ListenAndServe()
	cancelSrv()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return <-ch
}
