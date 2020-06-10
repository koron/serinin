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
	//		log.Printf("goroutine:%d fail:%d=(%d+%d) accept:%d", g, f0+f, f0, f, c)
	log.Printf("goroutine:%d fail:%d accept:%d heap:%d stack:%d", ngo, st.WorkerFail+st.InquireFail, st.Inquire, m.HeapInuse, m.StackInuse)
}

func run(ctx context.Context) error {
	var (
		monitor int
	)
	flag.IntVar(&monitor, "monitor", 0, "enable monitoring (poll system metric in each N's second)")
	flag.Parse()

	c, err := seri.LoadConfig("serinin_config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
