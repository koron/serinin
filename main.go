package main

import (
	"context"
	"log"

	"github.com/koron/serinin/seri"
)

func main() {
	err := run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	s, err := seri.New(&seri.Config{
		Addr: ":8000",
	})
	if err != nil {
		return err
	}
	return s.Serve(ctx)
}
