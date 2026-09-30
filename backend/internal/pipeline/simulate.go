package pipeline

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// Status of a step, stage, or run.
const (
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusSkipped    = "skipped"
	StatusRolledBack = "rolled-back"
)

// Options control one simulated run.
type Options struct {
	// Seed makes flaky steps reproducible.
	Seed int64
	// WarmCache makes steps with CacheHitDuration use it.
	WarmCache bool
	// FailStep forces the named step to fail on every attempt.
	FailStep string
	// BadRelease makes the deployed version unhealthy, so deploy analysis
	// fails and the rollout is rolled back.
	BadRelease bool
}

// Event is one entry of the run timeline; times are offsets on the virtual
// clock.
type Event struct {
	At      time.Duration `json:"at"`
	Stage   string        `json:"stage"`
	Message string        `json:"message"`
}

// StepResult is the outcome of a step.
type StepResult struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Attempts int           `json:"attempts"`
	Start    time.Duration `json:"start"`
	End      time.Duration `json:"end"`
}

// StageResult is the outcome of a stage.
type StageResult struct {
	Name   string        `json:"name"`
	Status string        `json:"status"`
	Start  time.Duration `json:"start"`
	End    time.Duration `json:"end"`
	Steps  []StepResult  `json:"steps,omitempty"`
}

// Run is the outcome of a simulated pipeline run.
type Run struct {
	Pipeline string        `json:"pipeline"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Stages   []StageResult `json:"stages"`
	Events   []Event       `json:"events"`
}

// Simulate plays the pipeline on a virtual clock. A failed stage stops the
// run; later stages are reported as skipped.
func Simulate(p *Pipeline, opt Options) (Run, error) {
	if err := p.Validate(); err != nil {
		return Run{}, err
	}
	rng := rand.New(rand.NewSource(opt.Seed))
	run := Run{Pipeline: p.Metadata.Name, Status: StatusSucceeded}
	var clock time.Duration
	stopped := false

	for _, st := range p.Spec.Stages {
		if stopped {
			run.Stages = append(run.Stages, StageResult{Name: st.Name, Status: StatusSkipped, Start: clock, End: clock})
			continue
		}
		var res StageResult
		if st.Deploy != nil {
			res = simulateDeploy(&run, st, clock, opt)
		} else {
			res = simulateSteps(&run, st, clock, opt, rng)
		}
		clock = res.End
		run.Stages = append(run.Stages, res)
		switch res.Status {
		case StatusFailed:
			run.Status = StatusFailed
			stopped = true
		case StatusRolledBack:
			run.Status = StatusRolledBack
			stopped = true
		}
	}
	run.Duration = clock
	// Parallel steps log in declaration order; present the timeline by time.
	sort.SliceStable(run.Events, func(i, j int) bool { return run.Events[i].At < run.Events[j].At })
	return run, nil
}

func (r *Run) log(at time.Duration, stage, format string, a ...any) {
	r.Events = append(r.Events, Event{At: at, Stage: stage, Message: fmt.Sprintf(format, a...)})
}

func simulateSteps(run *Run, st Stage, start time.Duration, opt Options, rng *rand.Rand) StageResult {
	res := StageResult{Name: st.Name, Status: StatusSucceeded, Start: start}
	run.log(start, st.Name, "stage started")
	cursor := start
	end := start
	for _, s := range st.Steps {
		stepStart := cursor
		if st.Parallel {
			stepStart = start
		}
		sr := runStep(run, st.Name, s, stepStart, opt, rng)
		res.Steps = append(res.Steps, sr)
		if sr.End > end {
			end = sr.End
		}
		if !st.Parallel {
			cursor = sr.End
		}
		if sr.Status == StatusFailed {
			res.Status = StatusFailed
			if !st.Parallel {
				break
			}
		}
	}
	res.End = end
	run.log(end, st.Name, "stage %s", res.Status)
	return res
}

func runStep(run *Run, stage string, s Step, start time.Duration, opt Options, rng *rand.Rand) StepResult {
	sr := StepResult{Name: s.Name, Start: start}
	d := time.Duration(s.Duration)
	if opt.WarmCache && s.CacheHitDuration > 0 {
		d = time.Duration(s.CacheHitDuration)
	}
	at := start
	for attempt := 1; attempt <= s.Retries+1; attempt++ {
		sr.Attempts = attempt
		run.log(at, stage, "%s: attempt %d started", s.Name, attempt)
		at += d
		failed := opt.FailStep == s.Name || (s.FlakyPercent > 0 && rng.Float64()*100 < s.FlakyPercent)
		if !failed {
			sr.Status = StatusSucceeded
			run.log(at, stage, "%s: succeeded", s.Name)
			sr.End = at
			return sr
		}
		run.log(at, stage, "%s: attempt %d failed", s.Name, attempt)
	}
	sr.Status = StatusFailed
	sr.End = at
	run.log(at, stage, "%s: failed after %d attempt(s)", s.Name, sr.Attempts)
	return sr
}

func simulateDeploy(run *Run, st Stage, start time.Duration, opt Options) StageResult {
	d := st.Deploy
	res := StageResult{Name: st.Name, Status: StatusSucceeded, Start: start}
	at := start
	run.log(at, st.Name, "deploy started (%s)", d.Strategy)
	step := time.Duration(d.StepDuration)
	analysis := time.Duration(d.AnalysisDuration)

	switch d.Strategy {
	case StrategyRolling:
		// The first new replica reveals a bad release: it never becomes
		// ready, the rollout stalls, and the deployment rolls back.
		for i := 1; i <= d.Replicas; i++ {
			at += step
			if opt.BadRelease {
				run.log(at, st.Name, "replica %d/%d failed its readiness check; rollout halted", i, d.Replicas)
				at += step
				run.log(at, st.Name, "rolled back to the previous version (old replicas kept serving)")
				res.Status = StatusRolledBack
				res.End = at
				return res
			}
			run.log(at, st.Name, "replica %d/%d updated and ready", i, d.Replicas)
		}
	case StrategyCanary:
		for _, w := range d.CanarySteps {
			run.log(at, st.Name, "canary receives %d%% of traffic", w)
			at += analysis
			if opt.BadRelease {
				run.log(at, st.Name, "analysis at %d%% failed: error rate above threshold", w)
				run.log(at, st.Name, "traffic returned to the stable version (0%% canary)")
				res.Status = StatusRolledBack
				res.End = at
				return res
			}
			run.log(at, st.Name, "analysis at %d%% passed", w)
		}
	case StrategyBlueGreen:
		at += step
		run.log(at, st.Name, "idle color provisioned and healthy")
		run.log(at, st.Name, "traffic switched to the new color")
		at += analysis
		if opt.BadRelease {
			run.log(at, st.Name, "post-switch analysis failed: switching back to the previous color")
			res.Status = StatusRolledBack
			res.End = at
			return res
		}
		run.log(at, st.Name, "post-switch analysis passed; old color retained for rollback")
	}
	res.End = at
	run.log(at, st.Name, "deploy succeeded")
	return res
}
