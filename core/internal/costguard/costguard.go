// Package costguard evaluates a Terraform plan (terraform show -json) against
// declared cost-guard rules before anything is applied: an estimated monthly
// ceiling, denied resource types, required cost-tracking tags, and a maximum
// experiment lifetime. It never contacts a cloud provider; estimates come
// from the price table in the rules file, which the user maintains.
package costguard

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

// HoursPerMonth converts hourly prices to monthly estimates.
const HoursPerMonth = 730

// TTLTag is the tag or label carrying the experiment lifetime in hours.
const TTLTag = "forgelab-ttl-hours"

// Rules is the cost-guard configuration (kind: CostGuard).
type Rules struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       Spec   `json:"spec"`
}

// Spec is the rule body.
type Spec struct {
	// MaxMonthlyUSD is the ceiling for the estimated monthly cost.
	MaxMonthlyUSD float64 `json:"maxMonthlyUsd"`
	// MaxTTLHours is the longest allowed experiment lifetime.
	MaxTTLHours int `json:"maxTtlHours"`
	// FailOnUnpriced turns resources with no price entry into violations.
	FailOnUnpriced bool `json:"failOnUnpriced"`
	// RequiredTags must be present on every taggable resource.
	RequiredTags []string `json:"requiredTags"`
	// DeniedTypes are resource types that must not appear in a plan.
	DeniedTypes []string `json:"deniedTypes"`
	// FreeTypes are resource types known to cost nothing on their own.
	FreeTypes []string `json:"freeTypes"`
	// Pricing prices the resource types that cost money.
	Pricing []Price `json:"pricing"`
}

// Price describes how to cost one resource type.
type Price struct {
	Type string `json:"type"`
	// FixedHourlyUSD is a flat hourly price per resource.
	FixedHourlyUSD float64 `json:"fixedHourlyUsd,omitempty"`
	// InstanceAttribute is a dotted path (for example instance_types.0) whose
	// value selects the entry of HourlyUSD.
	InstanceAttribute string             `json:"instanceAttribute,omitempty"`
	HourlyUSD         map[string]float64 `json:"hourlyUsd,omitempty"`
	// CountAttribute is a dotted path to the number of instances (default 1).
	CountAttribute string `json:"countAttribute,omitempty"`
	// CountMultiplier multiplies the count (for example zones of a regional
	// node pool).
	CountMultiplier float64 `json:"countMultiplier,omitempty"`
	// PerGBAttribute is a dotted path to a size in GB multiplied by
	// FixedHourlyUSD instead of a flat price.
	PerGBAttribute string `json:"perGbAttribute,omitempty"`
	// SpotAttribute and SpotValue mark discounted capacity; SpotFactor is the
	// share of the on-demand price paid.
	SpotAttribute string `json:"spotAttribute,omitempty"`
	SpotValue     string `json:"spotValue,omitempty"`
	// SpotBool marks SpotAttribute as a boolean that is true for spot capacity.
	SpotBool   bool    `json:"spotBool,omitempty"`
	SpotFactor float64 `json:"spotFactor,omitempty"`
}

// LoadRules reads and validates a rules file.
func LoadRules(path string) (*Rules, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules %q: %w", path, err)
	}
	return ParseRules(raw)
}

// ParseRules validates YAML rules.
func ParseRules(raw []byte) (*Rules, error) {
	var r Rules
	if err := yaml.UnmarshalStrict(raw, &r); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	switch {
	case r.APIVersion != "forgelab/v1":
		return nil, fmt.Errorf("apiVersion must be forgelab/v1, got %q", r.APIVersion)
	case r.Kind != "CostGuard":
		return nil, fmt.Errorf("kind must be CostGuard, got %q", r.Kind)
	case r.Spec.MaxMonthlyUSD <= 0:
		return nil, fmt.Errorf("spec.maxMonthlyUsd must be > 0")
	case r.Spec.MaxTTLHours < 0:
		return nil, fmt.Errorf("spec.maxTtlHours must not be negative")
	}
	seen := map[string]bool{}
	for _, p := range r.Spec.Pricing {
		if p.Type == "" {
			return nil, fmt.Errorf("pricing entry without type")
		}
		if seen[p.Type] {
			return nil, fmt.Errorf("duplicate pricing entry for %s", p.Type)
		}
		seen[p.Type] = true
		if p.InstanceAttribute != "" && len(p.HourlyUSD) == 0 {
			return nil, fmt.Errorf("pricing %s: instanceAttribute needs hourlyUsd", p.Type)
		}
		if p.InstanceAttribute == "" && p.FixedHourlyUSD < 0 {
			return nil, fmt.Errorf("pricing %s: negative price", p.Type)
		}
		if p.SpotFactor < 0 || p.SpotFactor > 1 {
			return nil, fmt.Errorf("pricing %s: spotFactor must be in [0, 1]", p.Type)
		}
	}
	return &r, nil
}

// Line is the cost estimate of one resource.
type Line struct {
	Address    string
	Type       string
	MonthlyUSD float64
	Note       string
}

// Finding is a rule violation or a warning.
type Finding struct {
	Rule    string
	Address string
	Message string
}

// Report is the outcome of evaluating a plan.
type Report struct {
	MonthlyUSD float64
	Lines      []Line
	Violations []Finding
	Warnings   []Finding
}

// OK reports whether the plan passes every rule.
func (r Report) OK() bool { return len(r.Violations) == 0 }

type plan struct {
	ResourceChanges []struct {
		Address string `json:"address"`
		Type    string `json:"type"`
		Change  struct {
			Actions []string       `json:"actions"`
			After   map[string]any `json:"after"`
		} `json:"change"`
	} `json:"resource_changes"`
}

// taggableAttributes are the attribute names that hold tags or labels.
var taggableAttributes = []string{"tags_all", "tags", "effective_labels", "labels", "resource_labels", "user_labels"}

// Evaluate checks a plan (terraform show -json output) against the rules.
func Evaluate(rules *Rules, planJSON []byte) (Report, error) {
	var p plan
	if err := json.Unmarshal(planJSON, &p); err != nil {
		return Report{}, fmt.Errorf("parse plan: %w", err)
	}
	spec := rules.Spec
	prices := map[string]Price{}
	for _, pr := range spec.Pricing {
		prices[pr.Type] = pr
	}
	denied := toSet(spec.DeniedTypes)
	free := toSet(spec.FreeTypes)

	var rep Report
	for _, rc := range p.ResourceChanges {
		if isOnlyDelete(rc.Change.Actions) || isNoOpRead(rc.Change.Actions) {
			continue
		}
		after := rc.Change.After
		if denied[rc.Type] {
			rep.Violations = append(rep.Violations, Finding{"denied-type", rc.Address, fmt.Sprintf("resource type %s is denied by the cost guard", rc.Type)})
		}
		rep.checkTags(spec, rc.Address, after)

		pr, priced := prices[rc.Type]
		switch {
		case priced:
			monthly, note, err := estimate(pr, after)
			if err != nil {
				rep.Violations = append(rep.Violations, Finding{"pricing", rc.Address, err.Error()})
				continue
			}
			rep.MonthlyUSD += monthly
			rep.Lines = append(rep.Lines, Line{rc.Address, rc.Type, monthly, note})
		case free[rc.Type]:
		default:
			f := Finding{"unpriced", rc.Address, fmt.Sprintf("no price for %s; add it to pricing or freeTypes", rc.Type)}
			if spec.FailOnUnpriced {
				rep.Violations = append(rep.Violations, f)
			} else {
				rep.Warnings = append(rep.Warnings, f)
			}
		}
	}
	if rep.MonthlyUSD > spec.MaxMonthlyUSD {
		rep.Violations = append(rep.Violations, Finding{"max-monthly", "", fmt.Sprintf("estimated $%.2f/month exceeds the $%.2f ceiling", rep.MonthlyUSD, spec.MaxMonthlyUSD)})
	}
	sort.Slice(rep.Lines, func(i, j int) bool { return rep.Lines[i].MonthlyUSD > rep.Lines[j].MonthlyUSD })
	return rep, nil
}

func (rep *Report) checkTags(spec Spec, address string, after map[string]any) {
	if len(spec.RequiredTags) == 0 && spec.MaxTTLHours == 0 {
		return
	}
	tags, taggable := collectTags(after)
	if !taggable {
		return
	}
	for _, want := range spec.RequiredTags {
		if _, ok := tags[want]; !ok {
			rep.Violations = append(rep.Violations, Finding{"required-tag", address, fmt.Sprintf("missing required tag/label %q", want)})
		}
	}
	if spec.MaxTTLHours > 0 {
		if v, ok := tags[TTLTag]; ok {
			hours, err := strconv.Atoi(v)
			switch {
			case err != nil || hours < 1:
				rep.Violations = append(rep.Violations, Finding{"ttl", address, fmt.Sprintf("%s must be a positive number of hours, got %q", TTLTag, v)})
			case hours > spec.MaxTTLHours:
				rep.Violations = append(rep.Violations, Finding{"ttl", address, fmt.Sprintf("%s=%d exceeds the maximum of %d hours", TTLTag, hours, spec.MaxTTLHours)})
			}
		}
	}
}

// collectTags merges every tag-like attribute of a resource. taggable is
// false when the resource has none of the known attributes (for example an
// IAM policy attachment), so it is not held to the tagging rule.
func collectTags(after map[string]any) (tags map[string]string, taggable bool) {
	tags = map[string]string{}
	for _, attr := range taggableAttributes {
		v, ok := after[attr]
		if !ok {
			continue
		}
		taggable = true
		m, _ := v.(map[string]any)
		for k, val := range m {
			tags[k] = fmt.Sprint(val)
		}
	}
	return tags, taggable
}

func estimate(pr Price, after map[string]any) (monthly float64, note string, err error) {
	hourly := pr.FixedHourlyUSD
	note = "fixed"
	switch {
	case pr.InstanceAttribute != "":
		key, ok := lookupString(after, pr.InstanceAttribute)
		if !ok {
			return 0, "", fmt.Errorf("cannot read %s to price %s", pr.InstanceAttribute, pr.Type)
		}
		price, known := pr.HourlyUSD[key]
		if !known {
			return 0, "", fmt.Errorf("no price for %s %q", pr.Type, key)
		}
		hourly = price
		note = key
	case pr.PerGBAttribute != "":
		gb, ok := lookupNumber(after, pr.PerGBAttribute)
		if !ok {
			return 0, "", fmt.Errorf("cannot read %s to price %s", pr.PerGBAttribute, pr.Type)
		}
		hourly = pr.FixedHourlyUSD * gb
		note = fmt.Sprintf("%g GB", gb)
	}

	count := 1.0
	if pr.CountAttribute != "" {
		n, ok := lookupNumber(after, pr.CountAttribute)
		if !ok {
			return 0, "", fmt.Errorf("cannot read %s to price %s", pr.CountAttribute, pr.Type)
		}
		count = n
	}
	if pr.CountMultiplier > 0 {
		count *= pr.CountMultiplier
	}
	if pr.SpotAttribute != "" {
		v, _ := lookupString(after, pr.SpotAttribute)
		spot := v == pr.SpotValue
		if pr.SpotBool {
			b, _ := lookupBool(after, pr.SpotAttribute)
			spot = b
		}
		if spot && pr.SpotFactor > 0 {
			hourly *= pr.SpotFactor
			note += " spot"
		}
	}
	return hourly * count * HoursPerMonth, fmt.Sprintf("%s x%g", note, count), nil
}

func isOnlyDelete(actions []string) bool {
	return len(actions) == 1 && actions[0] == "delete"
}

func isNoOpRead(actions []string) bool {
	return len(actions) == 1 && actions[0] == "read"
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

func lookup(v any, path string) (any, bool) {
	cur := v
	for _, part := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			cur = node[i]
		default:
			return nil, false
		}
	}
	return cur, cur != nil
}

func lookupString(v any, path string) (string, bool) {
	x, ok := lookup(v, path)
	if !ok {
		return "", false
	}
	s, ok := x.(string)
	return s, ok
}

func lookupNumber(v any, path string) (float64, bool) {
	x, ok := lookup(v, path)
	if !ok {
		return 0, false
	}
	n, ok := x.(float64)
	return n, ok
}

func lookupBool(v any, path string) (bool, bool) {
	x, ok := lookup(v, path)
	if !ok {
		return false, false
	}
	b, ok := x.(bool)
	return b, ok
}
