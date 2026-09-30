// Package pipeline simulates CI/CD pipelines (build, test, deploy) on a
// virtual clock. Nothing is executed and nothing sleeps: a pipeline is a
// declared model whose durations, flakiness, and deploy strategy are played
// out deterministically, so a run is reproducible for the same pipeline,
// seed, and options (Phase 7).
package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"sigs.k8s.io/yaml"
)

// Deploy strategies.
const (
	StrategyRolling   = "rolling"
	StrategyCanary    = "canary"
	StrategyBlueGreen = "blue-green"
)

// Duration is a time.Duration that unmarshals from a string such as "45s".
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"45s\": %s", b)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON renders the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Pipeline is a declared pipeline (kind: Pipeline).
type Pipeline struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec Spec `json:"spec"`
}

// Spec is the pipeline body.
type Spec struct {
	Stages []Stage `json:"stages"`
}

// Stage is a group of steps. Steps run one after another, or all at once
// when Parallel is set. A deploy stage carries a Strategy.
type Stage struct {
	Name     string  `json:"name"`
	Parallel bool    `json:"parallel,omitempty"`
	Steps    []Step  `json:"steps,omitempty"`
	Deploy   *Deploy `json:"deploy,omitempty"`
}

// Step is one unit of work.
type Step struct {
	Name     string   `json:"name"`
	Duration Duration `json:"duration"`
	// CacheHitDuration replaces Duration when the run has a warm cache.
	CacheHitDuration Duration `json:"cacheHitDuration,omitempty"`
	// FlakyPercent is the chance in [0, 100) that an attempt fails at random.
	FlakyPercent float64 `json:"flakyPercent,omitempty"`
	// Retries is the number of extra attempts after a failure.
	Retries int `json:"retries,omitempty"`
}

// Deploy declares how a release is rolled out.
type Deploy struct {
	Strategy string `json:"strategy"`
	// Replicas is the fleet size (rolling and blue-green).
	Replicas int `json:"replicas,omitempty"`
	// StepDuration is the time to replace one replica (rolling) or to
	// provision the idle color (blue-green).
	StepDuration Duration `json:"stepDuration,omitempty"`
	// CanarySteps are the traffic percentages of a canary rollout.
	CanarySteps []int `json:"canarySteps,omitempty"`
	// AnalysisDuration is the observation window at each canary step or
	// after a blue-green cutover.
	AnalysisDuration Duration `json:"analysisDuration,omitempty"`
}

// Load reads and validates a pipeline file.
func Load(path string) (*Pipeline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline %q: %w", path, err)
	}
	return Parse(raw)
}

// Parse validates a YAML pipeline.
func Parse(raw []byte) (*Pipeline, error) {
	var p Pipeline
	if err := yaml.UnmarshalStrict(raw, &p); err != nil {
		return nil, fmt.Errorf("parse pipeline: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate reports whether the pipeline can be simulated.
func (p *Pipeline) Validate() error {
	if p.APIVersion != "forgelab/v1" {
		return fmt.Errorf("apiVersion must be forgelab/v1, got %q", p.APIVersion)
	}
	if p.Kind != "Pipeline" {
		return fmt.Errorf("kind must be Pipeline, got %q", p.Kind)
	}
	if p.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if len(p.Spec.Stages) == 0 {
		return fmt.Errorf("at least one stage is required")
	}
	seen := map[string]bool{}
	for _, st := range p.Spec.Stages {
		if st.Name == "" {
			return fmt.Errorf("stage name is required")
		}
		if seen[st.Name] {
			return fmt.Errorf("duplicate stage %q", st.Name)
		}
		seen[st.Name] = true
		if (len(st.Steps) == 0) == (st.Deploy == nil) {
			return fmt.Errorf("stage %q needs either steps or deploy (not both, not neither)", st.Name)
		}
		names := map[string]bool{}
		for _, s := range st.Steps {
			if s.Name == "" || names[s.Name] {
				return fmt.Errorf("stage %q: step names must be unique and non-empty", st.Name)
			}
			names[s.Name] = true
			if s.Duration <= 0 {
				return fmt.Errorf("step %q: duration must be > 0", s.Name)
			}
			if s.FlakyPercent < 0 || s.FlakyPercent >= 100 {
				return fmt.Errorf("step %q: flakyPercent must be in [0, 100)", s.Name)
			}
			if s.Retries < 0 {
				return fmt.Errorf("step %q: retries must not be negative", s.Name)
			}
		}
		if st.Deploy != nil {
			if err := st.Deploy.validate(st.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Deploy) validate(stage string) error {
	switch d.Strategy {
	case StrategyRolling:
		if d.Replicas < 1 || d.StepDuration <= 0 {
			return fmt.Errorf("stage %q: rolling needs replicas >= 1 and stepDuration > 0", stage)
		}
	case StrategyCanary:
		if len(d.CanarySteps) == 0 || d.AnalysisDuration <= 0 {
			return fmt.Errorf("stage %q: canary needs canarySteps and analysisDuration", stage)
		}
		prev := 0
		for _, w := range d.CanarySteps {
			if w <= prev || w > 100 {
				return fmt.Errorf("stage %q: canarySteps must increase and end at most at 100", stage)
			}
			prev = w
		}
		if prev != 100 {
			return fmt.Errorf("stage %q: canarySteps must end at 100", stage)
		}
	case StrategyBlueGreen:
		if d.StepDuration <= 0 || d.AnalysisDuration <= 0 {
			return fmt.Errorf("stage %q: blue-green needs stepDuration and analysisDuration", stage)
		}
	default:
		return fmt.Errorf("stage %q: unknown strategy %q", stage, d.Strategy)
	}
	return nil
}
