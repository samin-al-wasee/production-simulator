package chaos

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const validExperiment = `
apiVersion: forgelab/v1
kind: Experiment
metadata:
  name: db-outage
spec:
  hypothesis: the app reports degraded while the database is down and recovers
  target:
    container: forgelab-db
  fault:
    type: stop
    duration: 20s
  probes:
    - {name: healthy, url: "http://localhost:8080/healthz", phase: before, expectStatus: 200}
    - {name: degraded, url: "http://localhost:8080/healthz", phase: during, expectStatus: 503}
    - {name: recovered, url: "http://localhost:8080/healthz", phase: after, expectStatus: 200}
`

func mustParse(t *testing.T, doc string) *Experiment {
	t.Helper()
	e, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestParseValid(t *testing.T) {
	e := mustParse(t, validExperiment)
	if e.Metadata.Name != "db-outage" || time.Duration(e.Spec.Fault.Duration) != 20*time.Second || len(e.Spec.Probes) != 3 {
		t.Fatalf("experiment = %+v", e)
	}
}

func TestParseErrors(t *testing.T) {
	repl := func(old, new string) string { return strings.Replace(validExperiment, old, new, 1) }
	cases := map[string]string{
		"bad version":     repl("forgelab/v1", "v2"),
		"bad kind":        repl("kind: Experiment", "kind: X"),
		"no name":         repl("name: db-outage", "name: \"\""),
		"foreign target":  repl("forgelab-db", "postgres"),
		"path traversal":  repl("forgelab-db", "forgelab-db; rm -rf"),
		"unknown fault":   repl("type: stop", "type: explode"),
		"zero duration":   repl("duration: 20s", "duration: 0s"),
		"bad duration":    repl("duration: 20s", "duration: soon"),
		"bad phase":       repl("phase: before", "phase: whenever"),
		"bad status":      repl("expectStatus: 200}\n    - {name: degraded", "expectStatus: 42}\n    - {name: degraded"),
		"unknown field":   repl("hypothesis:", "surprise: 1\n  hypothesis:"),
		"latency missing": repl("type: stop", "type: latency"),
		"loss missing":    repl("type: stop", "type: packet-loss"),
		"cpu missing":     repl("type: stop", "type: cpu-throttle"),
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestPlan(t *testing.T) {
	e := mustParse(t, validExperiment)
	in, out, err := Plan(e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(in, " ") != "docker stop -t 1 forgelab-db" || strings.Join(out, " ") != "docker start forgelab-db" {
		t.Fatalf("stop plan = %v / %v", in, out)
	}

	e.Spec.Fault = Fault{Type: FaultLatency, Duration: Duration(time.Second), Latency: Duration(300 * time.Millisecond)}
	in, out, _ = Plan(e)
	if !strings.Contains(strings.Join(in, " "), "netem delay 300ms") || !strings.Contains(strings.Join(out, " "), "qdisc del dev eth0 root") {
		t.Fatalf("latency plan = %v / %v", in, out)
	}
	if !strings.Contains(strings.Join(in, " "), "--network container:forgelab-db") {
		t.Fatalf("netem must join the target namespace: %v", in)
	}

	e.Spec.Fault = Fault{Type: FaultPacketLoss, Duration: Duration(time.Second), LossPercent: 25}
	in, _, _ = Plan(e)
	if !strings.Contains(strings.Join(in, " "), "netem loss 25%") {
		t.Fatalf("loss plan = %v", in)
	}

	e.Spec.Fault = Fault{Type: FaultCPUThrottle, Duration: Duration(time.Second), CPUs: 0.25}
	in, out, _ = Plan(e)
	if strings.Join(in, " ") != "docker update --cpus 0.25 forgelab-db" || strings.Join(out, " ") != "docker update --cpus 0 forgelab-db" {
		t.Fatalf("cpu plan = %v / %v", in, out)
	}

	e.Spec.Fault = Fault{Type: FaultPause, Duration: Duration(time.Second)}
	in, out, _ = Plan(e)
	if in[1] != "pause" || out[1] != "unpause" {
		t.Fatalf("pause plan = %v / %v", in, out)
	}
}

type fakeExec struct {
	ran    []string
	failOn string
}

func (f *fakeExec) Run(_ context.Context, cmd Command) (string, error) {
	line := strings.Join(cmd, " ")
	f.ran = append(f.ran, line)
	if f.failOn != "" && strings.Contains(line, f.failOn) {
		return "boom", errors.New("exec failed")
	}
	return "", nil
}

// fakeProber returns statuses as a function of whether the fault is active.
type fakeProber struct {
	exec         *fakeExec
	healthy      int
	degraded     int
	recoverAfter int // polls after revert before returning healthy again
	polls        int
	body         string
}

func (p *fakeProber) Probe(_ context.Context, _ string) (Response, error) {
	status := p.status()
	return Response{Status: status, Body: p.body}, nil
}

func (p *fakeProber) status() int {
	faulted := false
	for _, r := range p.exec.ran {
		if strings.Contains(r, "docker stop") {
			faulted = true
		}
		if strings.Contains(r, "docker start") {
			faulted = false
			p.polls++
			if p.polls <= p.recoverAfter {
				return p.degraded
			}
		}
	}
	if faulted {
		return p.degraded
	}
	return p.healthy
}

func newEnv(ex *fakeExec, pr Prober, slept *time.Duration) Env {
	return Env{Exec: ex, Probe: pr, Sleep: func(d time.Duration) { *slept += d }}
}

func TestRunPasses(t *testing.T) {
	ex := &fakeExec{}
	var slept time.Duration
	rep, err := Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503, recoverAfter: 3}, &slept))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Passed || !rep.Reverted || len(rep.Steps) != 3 {
		t.Fatalf("report = %+v", rep)
	}
	if slept < 20*time.Second {
		t.Fatalf("fault duration not held, slept %v", slept)
	}
	if len(ex.ran) < 2 || !strings.Contains(ex.ran[0], "docker stop") || !strings.Contains(ex.ran[1], "docker start") {
		t.Fatalf("commands = %v", ex.ran)
	}
}

func TestRunSkipsInjectionWhenSteadyStateFails(t *testing.T) {
	ex := &fakeExec{}
	var slept time.Duration
	rep, err := Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 500, degraded: 500}, &slept))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Passed || len(ex.ran) != 0 {
		t.Fatalf("must not inject when unhealthy: passed=%v ran=%v", rep.Passed, ex.ran)
	}
}

func TestRunFailsWhenHypothesisWrong(t *testing.T) {
	ex := &fakeExec{}
	var slept time.Duration
	// The system never degrades: "during" probe expects 503 but sees 200.
	rep, _ := Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 200}, &slept))
	if rep.Passed || !rep.Reverted {
		t.Fatalf("report = %+v", rep)
	}
}

func TestRunFailsWhenNoRecovery(t *testing.T) {
	ex := &fakeExec{}
	var slept time.Duration
	rep, _ := Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503, recoverAfter: 1000}, &slept))
	if rep.Passed {
		t.Fatalf("recovery never happened but run passed: %+v", rep)
	}
}

func TestRunRevertsWhenInjectFails(t *testing.T) {
	ex := &fakeExec{failOn: "docker stop"}
	var slept time.Duration
	_, err := Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503}, &slept))
	if err == nil {
		t.Fatal("expected inject error")
	}
	last := ex.ran[len(ex.ran)-1]
	if !strings.Contains(last, "docker start") {
		t.Fatalf("failed injection must attempt revert, ran %v", ex.ran)
	}
}

func TestProbeBodyMatch(t *testing.T) {
	e := mustParse(t, validExperiment)
	e.Spec.Probes = []Probe{{Name: "body", URL: "http://x", Phase: PhaseBefore, ExpectStatus: 200, ExpectBody: "needle"}}
	ex := &fakeExec{}
	var slept time.Duration
	rep, _ := Run(context.Background(), e, newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503, body: "hay"}, &slept))
	if len(ex.ran) != 0 || len(rep.Steps) != 1 || rep.Steps[0].Passed {
		t.Fatalf("body mismatch must fail the steady state: %+v ran=%v", rep, ex.ran)
	}
	rep, _ = Run(context.Background(), e, newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503, body: "a needle here"}, &slept))
	if !rep.Steps[0].Passed || len(ex.ran) == 0 {
		t.Fatalf("body match should pass and proceed: %+v", rep)
	}
}

func TestRevertRunsExactlyOnce(t *testing.T) {
	ex := &fakeExec{}
	var slept time.Duration
	Run(context.Background(), mustParse(t, validExperiment), newEnv(ex, &fakeProber{exec: ex, healthy: 200, degraded: 503}, &slept))
	n := 0
	for _, r := range ex.ran {
		if strings.Contains(r, "docker start") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("revert ran %d times: %v", n, ex.ran)
	}
}
