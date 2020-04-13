package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/koron-go/sigctx"
	"github.com/koron/serinin/seri"
)

func main() {
	ctx, cancel := sigctx.WithCancelSignal(context.Background(), os.Interrupt)
	defer cancel()
	err := run(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	s, err := seri.New(&seri.Config{
		Addr:              ":8000",
		ShutdownTimeout:   30 * time.Second,
		HttpClientTimeout: 200 * time.Millisecond,
		Endpoints: map[string]seri.Endpoint{
			"dst1": {URL: "http://127.0.0.1:10000"},
			"dst2": {URL: "http://127.0.0.1:10001"},
			"dst3": {URL: "http://127.0.0.1:10002"},
			"dst4": {URL: "http://127.0.0.1:10003"},
			"dst5": {URL: "http://127.0.0.1:10004"},
			"dst6": {URL: "http://127.0.0.1:10005"},
		},
		Redis: seri.Redis{
			Addr:   "localhost:6379",
			Expire: 30 * time.Second,
		},
	})
	if err != nil {
		return err
	}
	return s.Serve(ctx)
}
