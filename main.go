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
	ngo := runtime.NumGoroutine()
	st := b.Stat()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("goroutine:%d fail:%d accept:%d heap:%d stack:%d", ngo, st.WorkerFail+st.InquireFail, st.Inquire, m.HeapInuse, m.StackInuse)
}

func run(ctx context.Context) error {
	var (
		monitor int
		worker  int
		handler int
	)
	flag.IntVar(&monitor, "monitor", 0, "enable monitoring (poll system metric in each N's second)")
	flag.IntVar(&worker, "worker", 0, "override worker_num configuration if larger than zero")
	flag.IntVar(&handler, "handler", 0, "override max_handlers configuration if larger than zero")
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
