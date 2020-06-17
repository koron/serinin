package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/koron/serinin/internal/seri"
)

func main() {
	err := run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	var storeType string
	flag.StringVar(&storeType, "storetype", "", "override store_type configuration if not empty")
	flag.Parse()
	if flag.NArg() == 0 {
	}

	c, err := seri.LoadConfig("serinin_config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if storeType != "" {
		log.Printf("[INFO] store_type is overridden: %s -> %s", c.StoreType, storeType)
		c.StoreType = storeType
	}
	st, err := seri.NewStorage(c)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("  ", "  ")
	for _, id := range flag.Args() {
		r, err := st.GetResponse(id)
		if err != nil {
			fmt.Printf("%s: failed: %s\n", id, err)
			continue
		}
		fmt.Printf("%s: ", id)
		enc.Encode(r)
		fmt.Println()

	}

	return nil
}
