package engine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RunConfig describes a single load test run.
type RunConfig struct {
	URL         string
	Method      string
	Headers     map[string]string
	Body        string
	Concurrency int
	Duration    time.Duration
}

// Result is the outcome of one HTTP request.
type Result struct {
	Latency    time.Duration // round-trip time
	StatusCode int           // 0 when the request never completed
	Err        error         // non-nil on transport error
}

// Run launches cfg.Concurrency workers that fire HTTP requests in a tight loop
// for cfg.Duration. Every request outcome is sent on the returned channel,
// which is closed once all workers have stopped.
func Run(cfg RunConfig) <-chan Result {
	results := make(chan Result, cfg.Concurrency*2)

	method := cfg.Method
	if method == "" {
		method = http.MethodGet
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)

	// Each worker gets its own client so connection pools don't serialize.
	newClient := func() *http.Client {
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Concurrency,
				MaxIdleConnsPerHost: cfg.Concurrency,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(ctx, &wg, newClient(), method, cfg, results)
	}

	// Close results once every worker has returned, and release the context.
	go func() {
		wg.Wait()
		cancel()
		close(results)
	}()

	return results
}

func worker(ctx context.Context, wg *sync.WaitGroup, client *http.Client, method string, cfg RunConfig, results chan<- Result) {
	defer wg.Done()

	for {
		// Stop as soon as the duration is up.
		select {
		case <-ctx.Done():
			return
		default:
		}

		res := fire(ctx, client, method, cfg)

		// Don't block on a full channel if we're already shutting down.
		select {
		case results <- res:
		case <-ctx.Done():
			return
		}
	}
}

func fire(ctx context.Context, client *http.Client, method string, cfg RunConfig) Result {
	var bodyReader io.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bodyReader)
	if err != nil {
		return Result{Err: err}
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Latency: latency, Err: err}
	}
	defer resp.Body.Close()

	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	return Result{Latency: latency, StatusCode: resp.StatusCode}
}
