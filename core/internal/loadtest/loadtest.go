// Package loadtest is an open-loop HTTP load generator. Requests are
// dispatched on a fixed schedule regardless of how slowly earlier ones
// complete, so latency is not hidden by coordinated omission; when the
// in-flight limit is reached the request is counted as dropped rather than
// delayed.
package loadtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Profile names the shape of the request rate over time.
type Profile string

const (
	// Constant holds RPS for the whole duration.
	Constant Profile = "constant"
	// Ramp moves linearly from RPS to RampTo.
	Ramp Profile = "ramp"
	// Spike holds RPS, then runs at RampTo for the middle third.
	Spike Profile = "spike"
)

// Config describes one load test.
type Config struct {
	URL         string
	Method      string
	RPS         float64
	RampTo      float64
	Duration    time.Duration
	Profile     Profile
	MaxInFlight int
	Timeout     time.Duration
}

// Validate reports whether the configuration can run.
func (c Config) Validate() error {
	switch {
	case c.URL == "":
		return fmt.Errorf("url is required")
	case c.RPS <= 0:
		return fmt.Errorf("rps must be > 0")
	case c.Duration <= 0:
		return fmt.Errorf("duration must be > 0")
	case c.MaxInFlight < 1:
		return fmt.Errorf("max in flight must be >= 1")
	case c.Profile != Constant && c.RampTo <= 0:
		return fmt.Errorf("ramp-to must be > 0 for profile %q", c.Profile)
	}
	switch c.Profile {
	case Constant, Ramp, Spike:
		return nil
	}
	return fmt.Errorf("unknown profile %q", c.Profile)
}

// rateAt returns the target request rate at offset t into the test.
func (c Config) rateAt(t time.Duration) float64 {
	switch c.Profile {
	case Ramp:
		f := float64(t) / float64(c.Duration)
		return c.RPS + (c.RampTo-c.RPS)*f
	case Spike:
		if t >= c.Duration/3 && t < 2*c.Duration/3 {
			return c.RampTo
		}
	}
	return c.RPS
}

// Schedule returns the send offset of every request. It is deterministic:
// the same Config always yields the same schedule.
func Schedule(c Config) []time.Duration {
	var out []time.Duration
	var t time.Duration
	for t < c.Duration {
		out = append(out, t)
		rate := c.rateAt(t)
		t += time.Duration(float64(time.Second) / rate)
	}
	return out
}

// Result summarizes a finished test.
type Result struct {
	Sent        int           `json:"sent"`
	Dropped     int           `json:"dropped"`
	Errors      int           `json:"errors"`
	Statuses    map[int]int   `json:"statuses"`
	Elapsed     time.Duration `json:"elapsed"`
	AchievedRPS float64       `json:"achievedRps"`
	Latency     Latency       `json:"latency"`
}

// Latency holds request latency percentiles.
type Latency struct {
	Min  time.Duration `json:"min"`
	Mean time.Duration `json:"mean"`
	P50  time.Duration `json:"p50"`
	P90  time.Duration `json:"p90"`
	P95  time.Duration `json:"p95"`
	P99  time.Duration `json:"p99"`
	Max  time.Duration `json:"max"`
}

// ErrorRate is the share of sent requests that failed at the transport level
// or returned a 5xx status.
func (r Result) ErrorRate() float64 {
	if r.Sent == 0 {
		return 0
	}
	bad := r.Errors
	for code, n := range r.Statuses {
		if code >= 500 {
			bad += n
		}
	}
	return float64(bad) / float64(r.Sent)
}

// Percentile returns the p-th percentile (0..100) of sorted durations using
// the nearest-rank method. sorted must be in ascending order.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p/100*float64(len(sorted)) + 0.999999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func summarize(lat []time.Duration) Latency {
	if len(lat) == 0 {
		return Latency{}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	var sum time.Duration
	for _, d := range lat {
		sum += d
	}
	return Latency{
		Min: lat[0], Mean: sum / time.Duration(len(lat)),
		P50: Percentile(lat, 50), P90: Percentile(lat, 90), P95: Percentile(lat, 95), P99: Percentile(lat, 99),
		Max: lat[len(lat)-1],
	}
}

// Run executes the load test against cfg.URL using client (nil uses a
// default client with cfg.Timeout). It stops early when ctx is cancelled.
func Run(ctx context.Context, cfg Config, client *http.Client) (Result, error) {
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		res      = Result{Statuses: map[int]int{}}
		lat      []time.Duration
		inflight = make(chan struct{}, cfg.MaxInFlight)
	)
	start := time.Now()
	for _, offset := range Schedule(cfg) {
		if wait := time.Until(start.Add(offset)); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
			}
		}
		if ctx.Err() != nil {
			break
		}
		select {
		case inflight <- struct{}{}:
		default:
			res.Dropped++
			continue
		}
		res.Sent++
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-inflight }()
			t0 := time.Now()
			req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, nil)
			var status int
			if err == nil {
				var resp *http.Response
				if resp, err = client.Do(req); err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					status = resp.StatusCode
				}
			}
			d := time.Since(t0)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Errors++
				return
			}
			res.Statuses[status]++
			lat = append(lat, d)
		}()
	}
	wg.Wait()
	res.Elapsed = time.Since(start)
	if res.Elapsed > 0 {
		res.AchievedRPS = float64(res.Sent) / res.Elapsed.Seconds()
	}
	res.Latency = summarize(lat)
	return res, nil
}
