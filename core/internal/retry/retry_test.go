package retry

import (
	"reflect"
	"testing"
	"time"
)

var testPolicy = Policy{MaxAttempts: 4, BaseDelay: time.Second, Factor: 2, MaxDelay: 5 * time.Second}

func TestDelayBackoffAndCap(t *testing.T) {
	want := []time.Duration{0, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for n, w := range want {
		if got := testPolicy.Delay(n); got != w {
			t.Errorf("Delay(%d) = %v, want %v", n, got, w)
		}
	}
}

func TestDelayNoCap(t *testing.T) {
	p := Policy{MaxAttempts: 10, BaseDelay: time.Second, Factor: 3}
	if got := p.Delay(4); got != 27*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestDecide(t *testing.T) {
	if d := testPolicy.Decide(1); d.Action != Retry || d.Delay != time.Second {
		t.Errorf("attempt 1: %+v", d)
	}
	if d := testPolicy.Decide(3); d.Action != Retry || d.Delay != 4*time.Second {
		t.Errorf("attempt 3: %+v", d)
	}
	if d := testPolicy.Decide(4); d.Action != DeadLetter {
		t.Errorf("attempt 4: %+v", d)
	}
	if Retry.String() != "retry" || DeadLetter.String() != "dead-letter" {
		t.Error("action names")
	}
}

func TestSimulateSucceedsAfterRetries(t *testing.T) {
	out, err := Simulate(testPolicy, 2)
	if err != nil {
		t.Fatal(err)
	}
	if out.Attempts != 3 || out.DeadLettered || out.Total != 3*time.Second ||
		!reflect.DeepEqual(out.Delays, []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestSimulateDeadLetters(t *testing.T) {
	out, _ := Simulate(testPolicy, 100)
	if !out.DeadLettered || out.Attempts != 4 || len(out.Delays) != 3 {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestSimulateNoFailures(t *testing.T) {
	out, _ := Simulate(testPolicy, 0)
	if out.Attempts != 1 || out.DeadLettered || len(out.Delays) != 0 {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestValidate(t *testing.T) {
	bad := []Policy{
		{MaxAttempts: 0, Factor: 1},
		{MaxAttempts: 1, Factor: 0.5},
		{MaxAttempts: 1, Factor: 1, BaseDelay: -1},
		{MaxAttempts: 1, Factor: 1, MaxDelay: -1},
	}
	for _, p := range bad {
		if p.Validate() == nil {
			t.Errorf("expected error for %+v", p)
		}
		if _, err := Simulate(p, 1); err == nil {
			t.Errorf("Simulate must reject %+v", p)
		}
	}
	if _, err := Simulate(testPolicy, -1); err == nil {
		t.Error("expected error for negative failures")
	}
}
