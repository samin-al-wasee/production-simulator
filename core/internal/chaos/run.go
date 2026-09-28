package chaos

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Executor runs an external command and returns its combined output.
type Executor interface {
	Run(ctx context.Context, cmd Command) (string, error)
}

// Response is what a probe observed.
type Response struct {
	Status int
	Body   string
}

// Prober performs an HTTP GET; a transport failure returns an error.
type Prober interface {
	Probe(ctx context.Context, url string) (Response, error)
}

// Env holds the collaborators of a run, injectable for tests.
type Env struct {
	Exec  Executor
	Probe Prober
	Sleep func(time.Duration)
	Log   io.Writer
}

// Step is the outcome of one probe.
type Step struct {
	Probe  string `json:"probe"`
	Phase  string `json:"phase"`
	Status int    `json:"status"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// Report is the outcome of a run.
type Report struct {
	Experiment string `json:"experiment"`
	Steps      []Step `json:"steps"`
	// Reverted is true once the revert command succeeded.
	Reverted bool `json:"reverted"`
	Passed   bool `json:"passed"`
}

const (
	defaultDuringWithin = 5 * time.Second
	defaultAfterWithin  = 30 * time.Second
	pollInterval        = time.Second
)

// Run executes the experiment: before probes, inject, during probes, wait out
// the fault duration, revert, after probes. The revert always runs once the
// fault was injected, even when a probe fails or ctx is cancelled.
func Run(ctx context.Context, e *Experiment, env Env) (Report, error) {
	rep := Report{Experiment: e.Metadata.Name}
	inject, revert, err := Plan(e)
	if err != nil {
		return rep, err
	}
	logf := func(format string, a ...any) {
		if env.Log != nil {
			fmt.Fprintf(env.Log, format+"\n", a...)
		}
	}
	logf("experiment %s: %s", e.Metadata.Name, e.Spec.Hypothesis)

	ok := env.runProbes(ctx, e, PhaseBefore, &rep, logf)
	if !ok {
		logf("steady state not met; fault not injected")
		return rep, nil
	}

	logf("inject %s on %s: %s", e.Spec.Fault.Type, e.Spec.Target.Container, strings.Join(inject, " "))
	if out, err := env.Exec.Run(ctx, inject); err != nil {
		// A partial injection may have happened; try to restore.
		env.Exec.Run(context.Background(), revert)
		return rep, fmt.Errorf("inject fault: %w: %s", err, strings.TrimSpace(out))
	}

	// Revert with a fresh context so a cancelled run still restores the target.
	// The deferred call is a safety net; the revert normally runs before the
	// after probes and must not run twice (for example `unpause` fails on a
	// container that is not paused).
	reverted := false
	doRevert := func() {
		if reverted {
			return
		}
		reverted = true
		logf("revert: %s", strings.Join(revert, " "))
		if _, err := env.Exec.Run(context.Background(), revert); err == nil {
			rep.Reverted = true
		}
	}
	defer doRevert()

	duringOK := env.runProbes(ctx, e, PhaseDuring, &rep, logf)
	env.sleepRemaining(time.Duration(e.Spec.Fault.Duration), logf)

	doRevert()
	afterOK := env.runProbes(ctx, e, PhaseAfter, &rep, logf)
	rep.Passed = duringOK && afterOK && rep.Reverted
	return rep, nil
}

func (env Env) sleepRemaining(d time.Duration, logf func(string, ...any)) {
	logf("holding fault for %s", d)
	env.Sleep(d)
}

func (env Env) runProbes(ctx context.Context, e *Experiment, phase string, rep *Report, logf func(string, ...any)) bool {
	all := true
	for _, p := range e.Spec.Probes {
		if p.Phase != phase {
			continue
		}
		within := time.Duration(p.Within)
		if within == 0 {
			switch phase {
			case PhaseDuring:
				within = defaultDuringWithin
			case PhaseAfter:
				within = defaultAfterWithin
			}
		}
		step := env.poll(ctx, p, within)
		rep.Steps = append(rep.Steps, step)
		verdict := "PASS"
		if !step.Passed {
			verdict = "FAIL"
			all = false
		}
		logf("  [%s] %s %s: %s", verdict, phase, p.Name, step.Detail)
	}
	return all
}

func (env Env) poll(ctx context.Context, p Probe, within time.Duration) Step {
	step := Step{Probe: p.Name, Phase: p.Phase}
	var waited time.Duration
	for {
		resp, err := env.Probe.Probe(ctx, p.URL)
		step.Status = resp.Status
		matched := err == nil && resp.Status == p.ExpectStatus && strings.Contains(resp.Body, p.ExpectBody)
		switch {
		case err != nil:
			step.Detail = fmt.Sprintf("want %d, got error: %v", p.ExpectStatus, err)
		case resp.Status != p.ExpectStatus:
			step.Detail = fmt.Sprintf("want %d, got %d", p.ExpectStatus, resp.Status)
		case !matched:
			step.Detail = fmt.Sprintf("status %d as expected but body does not contain %q", resp.Status, p.ExpectBody)
		default:
			step.Detail = fmt.Sprintf("want %d, got %d", p.ExpectStatus, resp.Status)
		}
		if matched {
			step.Passed = true
			return step
		}
		if waited >= within || ctx.Err() != nil {
			return step
		}
		env.Sleep(pollInterval)
		waited += pollInterval
	}
}

// DockerExecutor runs commands with os/exec.
type DockerExecutor struct{}

// Run implements Executor.
func (DockerExecutor) Run(ctx context.Context, cmd Command) (string, error) {
	out, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	return string(out), err
}

// HTTPProber probes with net/http.
type HTTPProber struct {
	Client *http.Client
}

// Probe implements Prober.
func (p HTTPProber) Probe(ctx context.Context, url string) (Response, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Response{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return Response{Status: resp.StatusCode, Body: string(body)}, nil
}
