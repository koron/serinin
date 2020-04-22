package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	c, err := seri.LoadConfig("serinin_config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	b, err := seri.NewBroker(c)
	if err != nil {
		return fmt.Errorf("failed to setup broker: %w", err)
	}
	return b.Serve(ctx)
}
