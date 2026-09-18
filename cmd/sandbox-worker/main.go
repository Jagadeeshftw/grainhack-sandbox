// Command sandbox-worker reads jobs from stdin, one per line, and processes
// them with a pool of workers. Each job calls a downstream service that is
// briefly unavailable while it warms up, and is retried under
// worker.DefaultPolicy.
//
//	printf 'a\nb\nc\n' | go run ./cmd/sandbox-worker --concurrency 2
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Jagadeeshftw/grainhack-sandbox/worker"
)

type config struct {
	concurrency int
	warmup      time.Duration
}

// parseFlags reads the command line. It returns an error for anything the
// worker cannot run with, so main can exit before starting anything.
func parseFlags(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("sandbox-worker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	concurrency := fs.Int("concurrency", 4, "number of jobs processed at once")
	warmup := fs.Duration("warmup", 300*time.Millisecond, "how long the downstream is unavailable after start")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if *concurrency < 1 {
		// Report it the way the flag package reports its own parse errors,
		// so main does not need to know which kind of error it got.
		err := fmt.Errorf("invalid value %d for flag --concurrency: must be at least 1", *concurrency)
		fmt.Fprintln(fs.Output(), err)
		fs.Usage()
		return config{}, err
	}
	return config{concurrency: *concurrency, warmup: *warmup}, nil
}

var errUnavailable = errors.New("downstream unavailable")

// downstream stands in for a service the worker depends on. It refuses
// every call until warmup has passed since it started.
type downstream struct {
	started time.Time
	warmup  time.Duration
}

func (d downstream) call(ctx context.Context, job string) error {
	if time.Since(d.started) < d.warmup {
		return errUnavailable
	}
	return ctx.Err()
}

// runWorkers starts n workers that take jobs from the channel until it is
// closed, and returns a function that waits for them all to finish. n must
// be at least 1: a pool with no workers would accept jobs and never run them.
func runWorkers(ctx context.Context, n int, jobs <-chan string, handle func(context.Context, string)) (wait func()) {
	if n < 1 {
		panic(fmt.Sprintf("runWorkers: n must be at least 1, got %d", n))
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				handle(ctx, job)
			}
		}()
	}
	return wg.Wait
}

func main() {
	cfg, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ds := downstream{started: time.Now(), warmup: cfg.warmup}
	jobs := make(chan string)
	wait := runWorkers(ctx, cfg.concurrency, jobs, func(ctx context.Context, job string) {
		err := worker.Retry(ctx, worker.DefaultPolicy, func(ctx context.Context) error {
			return ds.call(ctx, job)
		})
		if err != nil {
			log.Printf("job %q failed: %v", job, err)
			return
		}
		fmt.Printf("done: %s\n", job)
	})
	log.Printf("started %d workers", cfg.concurrency)

	in := bufio.NewScanner(os.Stdin)
feed:
	for in.Scan() {
		select {
		case jobs <- in.Text():
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wait()
}
