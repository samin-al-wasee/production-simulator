package loadtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduleConstant(t *testing.T) {
	s := Schedule(Config{RPS: 10, Duration: time.Second, Profile: Constant})
	if len(s) != 10 || s[0] != 0 || s[1] != 100*time.Millisecond {
		t.Fatalf("schedule = %v", s)
	}
}

func TestScheduleRampSendsMore(t *testing.T) {
	flat := len(Schedule(Config{RPS: 10, Duration: 10 * time.Second, Profile: Constant}))
	ramp := len(Schedule(Config{RPS: 10, RampTo: 30, Duration: 10 * time.Second, Profile: Ramp}))
	if ramp <= flat*3/2 {
		t.Fatalf("ramp sent %d, flat sent %d", ramp, flat)
	}
}

func TestScheduleSpikeMiddleThird(t *testing.T) {
	c := Config{RPS: 10, RampTo: 100, Duration: 3 * time.Second, Profile: Spike}
	var first, mid, last int
	for _, off := range Schedule(c) {
		switch {
		case off < time.Second:
			first++
		case off < 2*time.Second:
			mid++
		default:
			last++
		}
	}
	if first != 10 || last != 10 || mid < 90 {
		t.Fatalf("first=%d mid=%d last=%d", first, mid, last)
	}
}

func TestSchedulerIsDeterministic(t *testing.T) {
	c := Config{RPS: 7, RampTo: 20, Duration: 5 * time.Second, Profile: Ramp}
	a, b := Schedule(c), Schedule(c)
	if len(a) != len(b) {
		t.Fatal("lengths differ")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("offset %d differs", i)
		}
	}
}

func TestPercentile(t *testing.T) {
	var d []time.Duration
	for i := 1; i <= 100; i++ {
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	cases := map[float64]time.Duration{50: 50 * time.Millisecond, 95: 95 * time.Millisecond, 99: 99 * time.Millisecond, 100: 100 * time.Millisecond, 0: time.Millisecond}
	for p, want := range cases {
		if got := Percentile(d, p); got != want {
			t.Errorf("p%v = %v, want %v", p, got, want)
		}
	}
	if Percentile(nil, 50) != 0 {
		t.Error("empty input")
	}
}

func TestValidate(t *testing.T) {
	good := Config{URL: "http://x", RPS: 1, Duration: time.Second, MaxInFlight: 1, Profile: Constant}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Config{
		{RPS: 1, Duration: time.Second, MaxInFlight: 1, Profile: Constant},
		{URL: "x", Duration: time.Second, MaxInFlight: 1, Profile: Constant},
		{URL: "x", RPS: 1, MaxInFlight: 1, Profile: Constant},
		{URL: "x", RPS: 1, Duration: time.Second, Profile: Constant},
		{URL: "x", RPS: 1, Duration: time.Second, MaxInFlight: 1, Profile: Ramp},
		{URL: "x", RPS: 1, Duration: time.Second, MaxInFlight: 1, Profile: "zigzag", RampTo: 1},
	}
	for i, c := range bad {
		if c.Validate() == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestRunCountsStatusesAndLatency(t *testing.T) {
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1)%4 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}))
	defer srv.Close()

	res, err := Run(context.Background(), Config{URL: srv.URL, RPS: 200, Duration: 500 * time.Millisecond, Profile: Constant, MaxInFlight: 50}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent < 90 || res.Errors != 0 {
		t.Fatalf("sent=%d errors=%d", res.Sent, res.Errors)
	}
	if res.Statuses[500] == 0 || res.Statuses[200] == 0 {
		t.Fatalf("statuses = %v", res.Statuses)
	}
	if rate := res.ErrorRate(); rate < 0.2 || rate > 0.3 {
		t.Fatalf("error rate = %v", rate)
	}
	if res.Latency.P50 < 5*time.Millisecond || res.Latency.Max < res.Latency.P50 {
		t.Fatalf("latency = %+v", res.Latency)
	}
}

func TestRunDropsWhenSaturated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	res, err := Run(context.Background(), Config{URL: srv.URL, RPS: 100, Duration: 300 * time.Millisecond, Profile: Constant, MaxInFlight: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Dropped == 0 || res.Sent > 4 {
		t.Fatalf("sent=%d dropped=%d", res.Sent, res.Dropped)
	}
}

func TestRunCountsTransportErrors(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	res, err := Run(context.Background(), Config{URL: url, RPS: 50, Duration: 200 * time.Millisecond, Profile: Constant, MaxInFlight: 10}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Errors == 0 || res.ErrorRate() != 1 {
		t.Fatalf("errors=%d rate=%v", res.Errors, res.ErrorRate())
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Run(ctx, Config{URL: srv.URL, RPS: 10, Duration: 30 * time.Second, Profile: Constant, MaxInFlight: 5}, nil); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("did not stop on cancel")
	}
}
