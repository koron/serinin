package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
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

func logSystemMetrics(b *seri.Broker) {
	st := b.Stat()
	to := st.InquireTimeout + st.StoreTimeout
	if st.InquireTimeout > 0 {
		log.Printf("[WARN] %d requests are timeouted, check loads of the server and destinations", st.InquireTimeout)
	}
	if st.StoreTimeout > 0 {
		log.Printf("[WARN] storing %d results are timeouted, check load of the storage", st.StoreTimeout)
	}

	// verbose monitoring
	ngo := runtime.NumGoroutine()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("accept:%d fail:%d timeout:%d goroutine:%d heap:%d stack:%d", st.Inquire, st.WorkerFail+st.InquireFail, to, ngo, m.HeapInuse, m.StackInuse)
}

func run(ctx context.Context) error {
	var (
		monitor int
		worker  int
		handler int

		storeType string
	)
	flag.IntVar(&monitor, "monitor", 0, "enable monitoring (poll system metric in each N's second)")
	flag.IntVar(&worker, "worker", 0, "override worker_num configuration if larger than zero")
	flag.IntVar(&handler, "handler", 0, "override max_handlers configuration if larger than zero")
	flag.StringVar(&storeType, "storetype", "", "override store_type configuration if not empty")
	flag.Parse()

	c, err := seri.LoadConfig("serinin_config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if worker > 0 {
		if c.WorkerNum != 0 {
			log.Printf("[INFO] worker_num is overridden: %d -> %d", c.WorkerNum, worker)
		}
		c.WorkerNum = worker
	}
	if handler > 0 {
		if c.MaxHandlers != 0 {
			log.Printf("[INFO] max_handlers is overridden: %d -> %d", c.MaxHandlers, handler)
		}
		c.MaxHandlers = handler
	}
	if storeType != "" {
		log.Printf("[INFO] store_type is overridden: %s -> %s", c.StoreType, storeType)
		c.StoreType = storeType
	}

	b, err := seri.NewBroker(c)
	if err != nil {
		return fmt.Errorf("failed to setup broker: %w", err)
	}

	if monitor > 0 {
		interval := time.Second * time.Duration(monitor)
		go func() {
			for {
				logSystemMetrics(b)
				time.Sleep(interval)
			}
		}()
	}

	return b.Serve(ctx)
}
