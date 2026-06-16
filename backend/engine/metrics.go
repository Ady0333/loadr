package engine

import (
	"sort"
	"time"
)

// MetricsSnapshot is a point-in-time view of an in-progress (or finished) run.
// All latencies are in milliseconds. It is JSON-tagged for streaming to the UI.
type MetricsSnapshot struct {
	ElapsedSeconds float64 `json:"elapsedSeconds"`
	Total          int     `json:"total"`
	Success        int     `json:"success"`
	Errors         int     `json:"errors"`
	ErrorRate      float64 `json:"errorRate"` // percentage, 0-100
	MinLatency     float64 `json:"minLatency"`
	AvgLatency     float64 `json:"avgLatency"`
	MaxLatency     float64 `json:"maxLatency"`
	P95Latency     float64 `json:"p95Latency"`
	RPS            float64 `json:"rps"`        // requests/sec over the whole run so far
	CurrentRPS     float64 `json:"currentRps"` // requests/sec over the last interval
	Final          bool    `json:"final"`      // true on the closing snapshot
}

// collector accumulates request outcomes and produces snapshots.
type collector struct {
	start time.Time

	total     int
	success   int
	errors    int
	sumMs     float64
	minMs     float64
	maxMs     float64
	latencies []float64 // every latency in ms, for percentile computation

	lastTotal int // total at the previous snapshot, for interval RPS
	lastEmit  time.Time
}

func newCollector() *collector {
	now := time.Now()
	return &collector{start: now, lastEmit: now}
}

func (c *collector) add(r Result) {
	c.total++
	ms := float64(r.Latency) / float64(time.Millisecond)

	// A request is successful when it completed with a non-5xx status and no
	// transport error.
	if r.Err != nil || r.StatusCode == 0 || r.StatusCode >= 500 {
		c.errors++
	} else {
		c.success++
	}

	// Latency stats only make sense for requests that actually round-tripped.
	if r.Err == nil {
		c.latencies = append(c.latencies, ms)
		c.sumMs += ms
		if c.minMs == 0 || ms < c.minMs {
			c.minMs = ms
		}
		if ms > c.maxMs {
			c.maxMs = ms
		}
	}
}

// snapshot computes the current metrics. final marks the closing emit.
func (c *collector) snapshot(final bool) MetricsSnapshot {
	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()

	var rps float64
	if elapsed > 0 {
		rps = float64(c.total) / elapsed
	}

	var currentRPS float64
	if interval := now.Sub(c.lastEmit).Seconds(); interval > 0 {
		currentRPS = float64(c.total-c.lastTotal) / interval
	}
	c.lastTotal = c.total
	c.lastEmit = now

	var avg float64
	if n := len(c.latencies); n > 0 {
		avg = c.sumMs / float64(n)
	}

	var errorRate float64
	if c.total > 0 {
		errorRate = float64(c.errors) / float64(c.total) * 100
	}

	return MetricsSnapshot{
		ElapsedSeconds: elapsed,
		Total:          c.total,
		Success:        c.success,
		Errors:         c.errors,
		ErrorRate:      errorRate,
		MinLatency:     c.minMs,
		AvgLatency:     avg,
		MaxLatency:     c.maxMs,
		P95Latency:     percentile(c.latencies, 95),
		RPS:            rps,
		CurrentRPS:     currentRPS,
		Final:          final,
	}
}

// percentile returns the p-th percentile (0-100) of the given latencies in ms.
// It sorts a copy so the collector's running slice is left untouched.
func percentile(latencies []float64, p float64) float64 {
	n := len(latencies)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, latencies)
	sort.Float64s(sorted)

	// Nearest-rank method.
	rank := int(p / 100 * float64(n))
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}

// Collect consumes results and emits a MetricsSnapshot once per second, plus a
// final snapshot when results closes. The returned channel is closed afterward.
func Collect(results <-chan Result) <-chan MetricsSnapshot {
	snapshots := make(chan MetricsSnapshot, 8)

	go func() {
		defer close(snapshots)

		c := newCollector()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case r, ok := <-results:
				if !ok {
					// Run finished: emit the final aggregate and stop.
					snapshots <- c.snapshot(true)
					return
				}
				c.add(r)
			case <-ticker.C:
				snapshots <- c.snapshot(false)
			}
		}
	}()

	return snapshots
}
