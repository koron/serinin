package main

import (
	"context"
	"log"
	"os"

	"github.com/koron-go/sigctx"
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
	s, err := NewServer(&Config{
		StartPort: 10001,
		Count:     6,
	})
	if err != nil {
		return err
	}
	return s.ServeAll(ctx)
}
