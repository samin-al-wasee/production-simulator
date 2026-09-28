// Package chaos runs declared fault-injection experiments against ForgeLab
// containers. An experiment states a hypothesis, verifies steady state,
// injects one fault, checks behavior during the fault, always reverts it, and
// verifies recovery. Planning is pure; only the injected Executor and Prober
// touch the outside world.
package chaos

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"sigs.k8s.io/yaml"
)

// Fault types.
const (
	FaultStop        = "stop"
	FaultPause       = "pause"
	FaultLatency     = "latency"
	FaultPacketLoss  = "packet-loss"
	FaultCPUThrottle = "cpu-throttle"
)

// Probe phases.
const (
	PhaseBefore = "before"
	PhaseDuring = "during"
	PhaseAfter  = "after"
)

// targetPattern restricts experiments to ForgeLab-owned containers so the
// tool cannot be pointed at anything else on the host.
var targetPattern = regexp.MustCompile(`^forgelab-[a-z0-9][a-z0-9-]*$`)

// Duration is a time.Duration that unmarshals from a string such as "20s".
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"20s\": %s", b)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// Experiment is a declared chaos experiment (kind: Experiment).
type Experiment struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec Spec `json:"spec"`
}

// Spec is the experiment body.
type Spec struct {
	Hypothesis string  `json:"hypothesis"`
	Target     Target  `json:"target"`
	Fault      Fault   `json:"fault"`
	Probes     []Probe `json:"probes"`
}

// Target names the container to disturb.
type Target struct {
	Container string `json:"container"`
}

// Fault describes the injected failure.
type Fault struct {
	Type     string   `json:"type"`
	Duration Duration `json:"duration"`
	// Latency is the added delay for FaultLatency.
	Latency Duration `json:"latency,omitempty"`
	// LossPercent is the packet loss for FaultPacketLoss, in (0, 100].
	LossPercent float64 `json:"lossPercent,omitempty"`
	// CPUs is the CPU limit for FaultCPUThrottle.
	CPUs float64 `json:"cpus,omitempty"`
}

// Probe is an HTTP check with an expected status.
type Probe struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Phase        string `json:"phase"`
	ExpectStatus int    `json:"expectStatus"`
	// ExpectBody, when set, must be a substring of the response body.
	ExpectBody string `json:"expectBody,omitempty"`
	// Within is how long to keep retrying before the probe fails. Defaults:
	// before 0, during 5s, after 30s.
	Within Duration `json:"within,omitempty"`
}

// Load reads and validates an experiment file.
func Load(path string) (*Experiment, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read experiment %q: %w", path, err)
	}
	return Parse(raw)
}

// Parse validates a YAML experiment.
func Parse(raw []byte) (*Experiment, error) {
	var e Experiment
	if err := yaml.UnmarshalStrict(raw, &e); err != nil {
		return nil, fmt.Errorf("parse experiment: %w", err)
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return &e, nil
}

// Validate reports whether the experiment is safe and complete.
func (e *Experiment) Validate() error {
	if e.APIVersion != "forgelab/v1" {
		return fmt.Errorf("apiVersion must be forgelab/v1, got %q", e.APIVersion)
	}
	if e.Kind != "Experiment" {
		return fmt.Errorf("kind must be Experiment, got %q", e.Kind)
	}
	if e.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if !targetPattern.MatchString(e.Spec.Target.Container) {
		return fmt.Errorf("target container %q must match %s", e.Spec.Target.Container, targetPattern)
	}
	f := e.Spec.Fault
	if f.Duration <= 0 {
		return fmt.Errorf("fault.duration must be > 0")
	}
	switch f.Type {
	case FaultStop, FaultPause:
	case FaultLatency:
		if f.Latency <= 0 {
			return fmt.Errorf("fault.latency must be > 0 for %s", f.Type)
		}
	case FaultPacketLoss:
		if f.LossPercent <= 0 || f.LossPercent > 100 {
			return fmt.Errorf("fault.lossPercent must be in (0, 100]")
		}
	case FaultCPUThrottle:
		if f.CPUs <= 0 {
			return fmt.Errorf("fault.cpus must be > 0")
		}
	default:
		return fmt.Errorf("unknown fault type %q", f.Type)
	}
	for i, p := range e.Spec.Probes {
		switch p.Phase {
		case PhaseBefore, PhaseDuring, PhaseAfter:
		default:
			return fmt.Errorf("probe %d: phase must be before, during, or after", i)
		}
		if p.URL == "" || p.Name == "" {
			return fmt.Errorf("probe %d: name and url are required", i)
		}
		if p.ExpectStatus < 100 || p.ExpectStatus > 599 {
			return fmt.Errorf("probe %q: expectStatus must be an HTTP status", p.Name)
		}
	}
	return nil
}

// Command is an external command: name followed by arguments.
type Command []string

// Plan returns the commands that inject and revert the fault. It has no side
// effects.
func Plan(e *Experiment) (inject, revert Command, err error) {
	if err := e.Validate(); err != nil {
		return nil, nil, err
	}
	c := e.Spec.Target.Container
	f := e.Spec.Fault
	switch f.Type {
	case FaultStop:
		return Command{"docker", "stop", "-t", "1", c}, Command{"docker", "start", c}, nil
	case FaultPause:
		return Command{"docker", "pause", c}, Command{"docker", "unpause", c}, nil
	case FaultCPUThrottle:
		return Command{"docker", "update", "--cpus", fmt.Sprintf("%g", f.CPUs), c}, Command{"docker", "update", "--cpus", "0", c}, nil
	case FaultLatency:
		return netem(c, "add", fmt.Sprintf("delay %dms", time.Duration(f.Latency).Milliseconds())), netem(c, "del", ""), nil
	default:
		return netem(c, "add", fmt.Sprintf("loss %g%%", f.LossPercent)), netem(c, "del", ""), nil
	}
}

// netem builds a command that runs tc inside the target's network namespace
// using a throwaway container, so target images need no extra tooling.
func netem(container, verb, args string) Command {
	tc := "tc qdisc " + verb + " dev eth0 root"
	if verb == "add" {
		tc += " netem " + args
	}
	return Command{
		"docker", "run", "--rm", "--network", "container:" + container, "--cap-add", "NET_ADMIN",
		"alpine:3.20", "sh", "-c", "apk add --no-cache iproute2-tc >/dev/null 2>&1 && " + tc,
	}
}
