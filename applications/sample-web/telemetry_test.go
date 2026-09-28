package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler() (http.Handler, *registry) {
	reg := newRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/work", handleWork)
	mux.Handle("/metrics", reg)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return instrument(mux, reg, logger, newTracer("test")), reg
}

func TestMetricsRecordRequests(t *testing.T) {
	h, _ := testHandler()
	for _, target := range []string{"/api/work", "/api/work?fail=1", "/nope"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", target, nil))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`http_requests_total{path="/api/work",status="200"} 1`,
		`http_requests_total{path="/api/work",status="500"} 1`,
		`http_requests_total{path="other",status="404"} 1`,
		`http_request_duration_seconds_count{path="/api/work"} 2`,
		`http_request_duration_seconds_bucket{path="/api/work",le="+Inf"} 2`,
		"http_requests_in_flight 1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n%s", want, body)
		}
	}
}

func TestTraceparentPropagation(t *testing.T) {
	h, _ := testHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/work", nil)
	req.Header.Set("Traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	h.ServeHTTP(rec, req)
	got := rec.Header().Get("Traceparent")
	if !strings.HasPrefix(got, "00-0af7651916cd43dd8448eb211c80319c-") {
		t.Fatalf("trace id not propagated: %q", got)
	}
	if strings.Contains(got, "b7ad6b7169203331") {
		t.Fatalf("response must carry a new span id: %q", got)
	}
}

func TestParseTraceparentRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "00-abc-def-01", "garbage"} {
		if id, _ := parseTraceparent(in); id != "" {
			t.Errorf("%q: unexpected trace id %q", in, id)
		}
	}
}

func TestWorkClampsDelay(t *testing.T) {
	rec := httptest.NewRecorder()
	handleWork(rec, httptest.NewRequest("GET", "/api/work?delay_ms=-5", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"delay_ms":0`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
