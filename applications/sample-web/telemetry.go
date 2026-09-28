package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Observability without third-party dependencies: Prometheus text metrics,
// structured JSON access logs carrying the trace id, and OTLP/HTTP JSON trace
// export (enabled by OTEL_EXPORTER_OTLP_ENDPOINT).

var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

type seriesKey struct {
	path   string
	status int
}

type histogram struct {
	buckets []uint64
	sum     float64
	count   uint64
}

type registry struct {
	mu       sync.Mutex
	requests map[seriesKey]uint64
	latency  map[string]*histogram
	inflight int64
}

func newRegistry() *registry {
	return &registry{requests: map[seriesKey]uint64{}, latency: map[string]*histogram{}}
}

func (r *registry) observe(path string, status int, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[seriesKey{path, status}]++
	h := r.latency[path]
	if h == nil {
		h = &histogram{buckets: make([]uint64, len(latencyBuckets))}
		r.latency[path] = h
	}
	secs := d.Seconds()
	for i, le := range latencyBuckets {
		if secs <= le {
			h.buckets[i]++
		}
	}
	h.sum += secs
	h.count++
}

func (r *registry) addInflight(delta int64) {
	r.mu.Lock()
	r.inflight += delta
	r.mu.Unlock()
}

func (r *registry) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder

	b.WriteString("# HELP http_requests_total Total HTTP requests.\n# TYPE http_requests_total counter\n")
	keys := make([]seriesKey, 0, len(r.requests))
	for k := range r.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		return keys[i].status < keys[j].status
	})
	for _, k := range keys {
		fmt.Fprintf(&b, "http_requests_total{path=%q,status=\"%d\"} %d\n", k.path, k.status, r.requests[k])
	}

	b.WriteString("# HELP http_request_duration_seconds HTTP request latency.\n# TYPE http_request_duration_seconds histogram\n")
	paths := make([]string, 0, len(r.latency))
	for p := range r.latency {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		h := r.latency[p]
		for i, le := range latencyBuckets {
			fmt.Fprintf(&b, "http_request_duration_seconds_bucket{path=%q,le=\"%g\"} %d\n", p, le, h.buckets[i])
		}
		fmt.Fprintf(&b, "http_request_duration_seconds_bucket{path=%q,le=\"+Inf\"} %d\n", p, h.count)
		fmt.Fprintf(&b, "http_request_duration_seconds_sum{path=%q} %g\n", p, h.sum)
		fmt.Fprintf(&b, "http_request_duration_seconds_count{path=%q} %d\n", p, h.count)
	}

	fmt.Fprintf(&b, "# HELP app_cache_hits_total Cache-aside hits.\n# TYPE app_cache_hits_total counter\napp_cache_hits_total %d\n", cacheHits.Load())
	fmt.Fprintf(&b, "# HELP app_cache_misses_total Cache-aside misses.\n# TYPE app_cache_misses_total counter\napp_cache_misses_total %d\n", cacheMisses.Load())
	fmt.Fprintf(&b, "# HELP app_rate_limited_total Requests rejected by the rate limiter.\n# TYPE app_rate_limited_total counter\napp_rate_limited_total %d\n", rateLimited.Load())

	b.WriteString("# HELP http_requests_in_flight Requests currently being served.\n# TYPE http_requests_in_flight gauge\n")
	fmt.Fprintf(&b, "http_requests_in_flight %d\n", r.inflight)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(b.String()))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// instrument records metrics, a structured access log line, and a trace span
// for every request. The route label uses the registered pattern so metric
// cardinality stays bounded.
func instrument(next http.Handler, reg *registry, logger *slog.Logger, tr *tracer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		traceID, parentID := parseTraceparent(r.Header.Get("Traceparent"))
		if traceID == "" {
			traceID = randomHex(16)
		}
		spanID := randomHex(8)
		w.Header().Set("Traceparent", "00-"+traceID+"-"+spanID+"-01")

		reg.addInflight(1)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		reg.addInflight(-1)

		route := routeLabel(r.URL.Path)
		dur := time.Since(start)
		reg.observe(route, rec.status, dur)
		logger.Info("request",
			"method", r.Method, "path", r.URL.Path, "route", route,
			"status", rec.status, "duration_ms", float64(dur.Microseconds())/1000,
			"trace_id", traceID, "span_id", spanID)
		tr.record(span{
			traceID: traceID, spanID: spanID, parentID: parentID,
			name: r.Method + " " + route, start: start, end: start.Add(dur), status: rec.status,
		})
	})
}

func routeLabel(path string) string {
	switch path {
	case "/", "/healthz", "/metrics", "/api/work", "/api/cache", "/api/limited", "/version":
		return path
	}
	return "other"
}

func parseTraceparent(h string) (traceID, parentID string) {
	parts := strings.Split(h, "-")
	if len(parts) != 4 || len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", ""
	}
	return parts[1], parts[2]
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type span struct {
	traceID, spanID, parentID string
	name                      string
	start, end                time.Time
	status                    int
}

// tracer exports finished spans to an OTLP/HTTP JSON endpoint in batches.
// A nil-endpoint tracer drops spans.
type tracer struct {
	endpoint string
	service  string
	ch       chan span
	client   *http.Client
}

func newTracer(service string) *tracer {
	endpoint := strings.TrimRight(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "/")
	t := &tracer{endpoint: endpoint, service: service, ch: make(chan span, 1024), client: &http.Client{Timeout: 2 * time.Second}}
	if endpoint != "" {
		go t.run()
	}
	return t
}

func (t *tracer) record(s span) {
	if t.endpoint == "" {
		return
	}
	select {
	case t.ch <- s:
	default:
	}
}

func (t *tracer) run() {
	tick := time.NewTicker(time.Second)
	var batch []span
	for {
		select {
		case s := <-t.ch:
			batch = append(batch, s)
			if len(batch) >= 100 {
				t.flush(batch)
				batch = nil
			}
		case <-tick.C:
			if len(batch) > 0 {
				t.flush(batch)
				batch = nil
			}
		}
	}
}

func (t *tracer) flush(batch []span) {
	spans := make([]map[string]any, 0, len(batch))
	for _, s := range batch {
		sp := map[string]any{
			"traceId":           s.traceID,
			"spanId":            s.spanID,
			"name":              s.name,
			"kind":              2,
			"startTimeUnixNano": strconv.FormatInt(s.start.UnixNano(), 10),
			"endTimeUnixNano":   strconv.FormatInt(s.end.UnixNano(), 10),
			"attributes": []map[string]any{
				{"key": "http.status_code", "value": map[string]any{"intValue": strconv.Itoa(s.status)}},
			},
		}
		if s.parentID != "" {
			sp["parentSpanId"] = s.parentID
		}
		if s.status >= 500 {
			sp["status"] = map[string]any{"code": 2}
		}
		spans = append(spans, sp)
	}
	payload := map[string]any{"resourceSpans": []map[string]any{{
		"resource": map[string]any{"attributes": []map[string]any{
			{"key": "service.name", "value": map[string]any{"stringValue": t.service}},
		}},
		"scopeSpans": []map[string]any{{"scope": map[string]any{"name": "sample-web"}, "spans": spans}},
	}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	resp, err := t.client.Post(t.endpoint+"/v1/traces", "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}
