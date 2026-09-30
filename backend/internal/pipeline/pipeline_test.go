package pipeline

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const testPipeline = `
apiVersion: forgelab/v1
kind: Pipeline
metadata:
  name: web-release
spec:
  stages:
    - name: build
      steps:
        - {name: compile, duration: 60s, cacheHitDuration: 10s}
        - {name: image, duration: 30s}
    - name: test
      parallel: true
      steps:
        - {name: unit, duration: 40s}
        - {name: integration, duration: 90s, flakyPercent: 50, retries: 2}
    - name: deploy
      deploy:
        strategy: canary
        canarySteps: [10, 50, 100]
        analysisDuration: 60s
`

func mustParse(t *testing.T, doc string) *Pipeline {
	t.Helper()
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSimulateHappyPath(t *testing.T) {
	// Seed chosen so the flaky step passes first time is not guaranteed; use a
	// non-flaky variant for exact timing.
	p := mustParse(t, strings.Replace(testPipeline, "flakyPercent: 50, ", "", 1))
	run, err := Simulate(p, Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	// build 90s + test max(40,90)=90s + canary 3*60s = 360s
	if run.Status != StatusSucceeded || run.Duration != 360*time.Second {
		t.Fatalf("status=%s duration=%v", run.Status, run.Duration)
	}
	if len(run.Stages) != 3 || run.Stages[1].Steps[1].Start != run.Stages[1].Start {
		t.Fatalf("parallel steps must start together: %+v", run.Stages[1])
	}
}

func TestWarmCacheIsFaster(t *testing.T) {
	p := mustParse(t, strings.Replace(testPipeline, "flakyPercent: 50, ", "", 1))
	cold, _ := Simulate(p, Options{})
	warm, _ := Simulate(p, Options{WarmCache: true})
	if cold.Duration-warm.Duration != 50*time.Second {
		t.Fatalf("cold=%v warm=%v", cold.Duration, warm.Duration)
	}
}

func TestSimulationIsDeterministic(t *testing.T) {
	p := mustParse(t, testPipeline)
	a, _ := Simulate(p, Options{Seed: 42})
	b, _ := Simulate(p, Options{Seed: 42})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed produced different runs")
	}
	different := false
	for seed := int64(0); seed < 50; seed++ {
		c, _ := Simulate(p, Options{Seed: seed})
		if c.Duration != a.Duration {
			different = true
			break
		}
	}
	if !different {
		t.Fatal("seeds never changed the outcome; flakiness is not applied")
	}
}

func TestRetriesRecoverFlakyStep(t *testing.T) {
	p := mustParse(t, testPipeline)
	sawRetry := false
	for seed := int64(0); seed < 200; seed++ {
		run, _ := Simulate(p, Options{Seed: seed})
		step := run.Stages[1].Steps[1]
		if step.Attempts > 1 && step.Status == StatusSucceeded {
			sawRetry = true
		}
		if step.Attempts > 3 {
			t.Fatalf("attempts %d exceed retries+1", step.Attempts)
		}
	}
	if !sawRetry {
		t.Fatal("no run recovered through a retry")
	}
}

func TestForcedFailureStopsPipeline(t *testing.T) {
	p := mustParse(t, testPipeline)
	run, _ := Simulate(p, Options{FailStep: "unit"})
	if run.Status != StatusFailed || run.Stages[1].Status != StatusFailed || run.Stages[2].Status != StatusSkipped {
		t.Fatalf("run = %+v", run)
	}
	if run.Stages[1].Steps[0].Attempts != 1 {
		t.Fatalf("unit has no retries: %+v", run.Stages[1].Steps[0])
	}
}

func TestBadReleaseRollsBackEachStrategy(t *testing.T) {
	cases := map[string]string{
		StrategyCanary: `strategy: canary
        canarySteps: [10, 100]
        analysisDuration: 30s`,
		StrategyRolling: `strategy: rolling
        replicas: 4
        stepDuration: 20s`,
		StrategyBlueGreen: `strategy: blue-green
        stepDuration: 40s
        analysisDuration: 30s`,
	}
	for name, deploy := range cases {
		doc := "apiVersion: forgelab/v1\nkind: Pipeline\nmetadata: {name: p}\nspec:\n  stages:\n    - name: deploy\n      deploy:\n        " + deploy + "\n"
		p := mustParse(t, doc)
		good, err := Simulate(p, Options{})
		if err != nil || good.Status != StatusSucceeded {
			t.Fatalf("%s good run: %v %+v", name, err, good)
		}
		bad, _ := Simulate(p, Options{BadRelease: true})
		if bad.Status != StatusRolledBack {
			t.Fatalf("%s bad release: status = %s", name, bad.Status)
		}
		if bad.Duration >= good.Duration && name != StrategyBlueGreen {
			t.Errorf("%s: a rollback at the first check should be faster than a full rollout (%v vs %v)", name, bad.Duration, good.Duration)
		}
	}
}

func TestCanaryRollbackHappensAtFirstStep(t *testing.T) {
	p := mustParse(t, testPipeline)
	run, _ := Simulate(p, Options{BadRelease: true, FailStep: ""})
	deploy := run.Stages[2]
	if deploy.Status != StatusRolledBack {
		// integration might have failed randomly; retry seeds until it passes
		for seed := int64(1); seed < 100 && deploy.Status != StatusRolledBack; seed++ {
			run, _ = Simulate(p, Options{BadRelease: true, Seed: seed})
			deploy = run.Stages[2]
		}
	}
	if deploy.Status != StatusRolledBack || deploy.End-deploy.Start != 60*time.Second {
		t.Fatalf("deploy = %+v", deploy)
	}
}

func TestParseErrors(t *testing.T) {
	head := "apiVersion: forgelab/v1\nkind: Pipeline\nmetadata: {name: p}\nspec:\n  stages:\n"
	cases := map[string]string{
		"version":       strings.Replace(testPipeline, "forgelab/v1", "v2", 1),
		"kind":          strings.Replace(testPipeline, "kind: Pipeline", "kind: X", 1),
		"no stages":     "apiVersion: forgelab/v1\nkind: Pipeline\nmetadata: {name: p}\nspec:\n  stages: []\n",
		"empty stage":   head + "    - name: a\n",
		"both":          head + "    - name: a\n      steps: [{name: s, duration: 1s}]\n      deploy: {strategy: rolling, replicas: 1, stepDuration: 1s}\n",
		"dup stage":     head + "    - {name: a, steps: [{name: s, duration: 1s}]}\n    - {name: a, steps: [{name: s, duration: 1s}]}\n",
		"dup step":      head + "    - name: a\n      steps: [{name: s, duration: 1s}, {name: s, duration: 1s}]\n",
		"zero duration": head + "    - name: a\n      steps: [{name: s, duration: 0s}]\n",
		"flaky":         head + "    - name: a\n      steps: [{name: s, duration: 1s, flakyPercent: 100}]\n",
		"strategy":      head + "    - name: a\n      deploy: {strategy: yolo}\n",
		"canary end":    head + "    - name: a\n      deploy: {strategy: canary, canarySteps: [10, 50], analysisDuration: 1s}\n",
		"canary order":  head + "    - name: a\n      deploy: {strategy: canary, canarySteps: [50, 10, 100], analysisDuration: 1s}\n",
		"rolling":       head + "    - name: a\n      deploy: {strategy: rolling}\n",
		"unknown field": head + "    - name: a\n      surprise: 1\n      steps: [{name: s, duration: 1s}]\n",
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestEventsAreChronological(t *testing.T) {
	p := mustParse(t, testPipeline)
	run, _ := Simulate(p, Options{Seed: 3})
	for i := 1; i < len(run.Events); i++ {
		if run.Events[i].At < run.Events[i-1].At {
			t.Fatalf("event %d (%v) precedes event %d (%v)", i, run.Events[i].At, i-1, run.Events[i-1].At)
		}
	}
}
