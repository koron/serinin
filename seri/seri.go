package seri

import (
	"context"
	"log"
	"net/http"
	"time"
)

type Seri struct {
	c Config
}

func New(c *Config) (*Seri, error) {
	return &Seri{
		c: c.Clone(),
	}, nil
}

func (s *Seri) Serve(ctx context.Context) error {
	srv := http.Server{
		Addr:    s.c.Addr,
		Handler: http.HandlerFunc(s.serveHTTP),
	}
	go func() {
		select {
		case <-ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			srv.Shutdown(ctx)
			cancel()
		}
	}()
	log.Printf("serving on: %s", s.c.Addr)
	return srv.ListenAndServe()
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
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte{})
	}
}

func (s *Seri) newReqid() string {
	// TODO:
	return ""
}

func (s *Seri) seriGet(w http.ResponseWriter, r *http.Request) {
	reqid := s.newReqid()
	_ = reqid
	// TODO:
}

func (s *Seri) seriPost(w http.ResponseWriter, r *http.Request) {
	reqid := s.newReqid()
	_ = reqid
	// TODO:
}
