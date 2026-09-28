// Package virtualcluster models the simulated production cluster that the
// Production View presents (ADR-0003): named node groups built from virtual
// hardware profiles. It is pure and deterministic.
package virtualcluster

import (
	"fmt"
	"os"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/capacity"
)

// Profile is a named virtual node size.
type Profile struct {
	CPUCores    float64 `json:"cpuCores"`
	MemoryBytes Bytes   `json:"memory"`
	DiskBytes   Bytes   `json:"disk"`
}

// Profiles are the built-in virtual hardware profiles.
var Profiles = map[string]Profile{
	"small":  {CPUCores: 2, MemoryBytes: 4 << 30, DiskBytes: 100 << 30},
	"medium": {CPUCores: 4, MemoryBytes: 16 << 30, DiskBytes: 500 << 30},
	"large":  {CPUCores: 16, MemoryBytes: 64 << 30, DiskBytes: 2 << 40},
	"xlarge": {CPUCores: 64, MemoryBytes: 256 << 30, DiskBytes: 8 << 40},
}

// NodeGroup is Count identical virtual nodes. Either Profile names a built-in
// profile or the inline Resources fields define a custom one.
type NodeGroup struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Profile string   `json:"profile,omitempty"`
	Custom  *Profile `json:"resources,omitempty"`
}

// Spec is the declared virtual cluster.
type Spec struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Name       string        `json:"name"`
	Nodes      []NodeGroup   `json:"nodes"`
	Database   *DatabaseSpec `json:"database,omitempty"`
}

// DatabaseSpec declares the virtual database tier.
type DatabaseSpec struct {
	MaxConnections int     `json:"maxConnections"`
	MaxQPS         float64 `json:"maxQps"`
	Storage        Bytes   `json:"storage"`
}

// Cluster is a validated virtual cluster.
type Cluster struct {
	Name     string
	Groups   []Group
	Database capacity.Database
}

// Group is a resolved node group.
type Group struct {
	Name    string
	Count   int
	Profile Profile
}

// Load reads and validates a cluster spec from a YAML file.
func Load(path string) (*Cluster, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster %q: %w", path, err)
	}
	return Parse(raw)
}

// Parse validates a YAML cluster spec.
func Parse(raw []byte) (*Cluster, error) {
	var spec Spec
	if err := yaml.UnmarshalStrict(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse cluster: %w", err)
	}
	return New(spec)
}

// New resolves and validates a Spec.
func New(spec Spec) (*Cluster, error) {
	if spec.APIVersion != "forgelab/v1" {
		return nil, fmt.Errorf("apiVersion must be forgelab/v1, got %q", spec.APIVersion)
	}
	if spec.Kind != "Cluster" {
		return nil, fmt.Errorf("kind must be Cluster, got %q", spec.Kind)
	}
	if spec.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(spec.Nodes) == 0 {
		return nil, fmt.Errorf("at least one node group is required")
	}
	c := &Cluster{Name: spec.Name}
	seen := map[string]bool{}
	for _, ng := range spec.Nodes {
		if ng.Name == "" {
			return nil, fmt.Errorf("node group name is required")
		}
		if seen[ng.Name] {
			return nil, fmt.Errorf("duplicate node group %q", ng.Name)
		}
		seen[ng.Name] = true
		if ng.Count < 1 {
			return nil, fmt.Errorf("node group %q: count must be >= 1", ng.Name)
		}
		p, err := resolveProfile(ng)
		if err != nil {
			return nil, err
		}
		c.Groups = append(c.Groups, Group{Name: ng.Name, Count: ng.Count, Profile: p})
	}
	if db := spec.Database; db != nil {
		if db.MaxConnections < 1 || db.MaxQPS <= 0 || db.Storage == 0 {
			return nil, fmt.Errorf("database needs positive maxConnections, maxQps, and storage")
		}
		c.Database = capacity.Database{MaxConnections: db.MaxConnections, MaxQPS: db.MaxQPS, StorageBytes: uint64(db.Storage)}
	}
	return c, nil
}

func resolveProfile(ng NodeGroup) (Profile, error) {
	switch {
	case ng.Profile != "" && ng.Custom != nil:
		return Profile{}, fmt.Errorf("node group %q: set either profile or resources, not both", ng.Name)
	case ng.Profile != "":
		p, ok := Profiles[ng.Profile]
		if !ok {
			return Profile{}, fmt.Errorf("node group %q: unknown profile %q (known: %v)", ng.Name, ng.Profile, ProfileNames())
		}
		return p, nil
	case ng.Custom != nil:
		p := *ng.Custom
		if p.CPUCores <= 0 || p.MemoryBytes == 0 || p.DiskBytes == 0 {
			return Profile{}, fmt.Errorf("node group %q: resources need positive cpuCores, memory, and disk", ng.Name)
		}
		return p, nil
	default:
		return Profile{}, fmt.Errorf("node group %q: profile or resources is required", ng.Name)
	}
}

// ProfileNames lists built-in profile names in sorted order.
func ProfileNames() []string {
	names := make([]string, 0, len(Profiles))
	for n := range Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Nodes returns the total virtual node count.
func (c *Cluster) Nodes() int {
	n := 0
	for _, g := range c.Groups {
		n += g.Count
	}
	return n
}

// Total returns the summed virtual resources of every node.
func (c *Cluster) Total() budget.Resources {
	var t budget.Resources
	for _, g := range c.Groups {
		n := float64(g.Count)
		t.CPUCores += g.Profile.CPUCores * n
		t.MemoryBytes += uint64(g.Profile.MemoryBytes) * uint64(g.Count)
		t.DiskBytes += uint64(g.Profile.DiskBytes) * uint64(g.Count)
	}
	return t
}

// Capacity returns the cluster's virtual capacity for the capacity model.
func (c *Cluster) Capacity() capacity.Capacity {
	return capacity.Capacity{Compute: c.Total(), Database: c.Database}
}
