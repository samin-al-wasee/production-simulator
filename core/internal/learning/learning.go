// Package learning models the ForgeLab learning path and a learner's
// progress through it. The path is declared in learning/path.yaml; progress
// is a small JSON file. Exercises complete manually, or automatically when a
// matching chaos experiment passes or a benchmark meets its SLOs.
package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"
)

// Evidence types.
const (
	EvidenceManual     = "manual"
	EvidenceExperiment = "experiment"
	EvidenceBenchmark  = "benchmark"
)

// Path is the declared learning path (kind: LearningPath).
type Path struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Stages []Stage `json:"stages"`
	} `json:"spec"`
}

// Stage is one milestone of the path.
type Stage struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Goal      string     `json:"goal"`
	Exercises []Exercise `json:"exercises"`
}

// Exercise is one checkable unit of learning.
type Exercise struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	How      string   `json:"how"`
	Evidence Evidence `json:"evidence"`
}

// Evidence says how an exercise is completed.
type Evidence struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// LoadPath reads and validates a path file.
func LoadPath(file string) (*Path, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read learning path %q: %w", file, err)
	}
	var p Path
	if err := yaml.UnmarshalStrict(raw, &p); err != nil {
		return nil, fmt.Errorf("parse learning path: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate reports whether the path is well formed.
func (p *Path) Validate() error {
	if p.APIVersion != "forgelab/v1" || p.Kind != "LearningPath" {
		return fmt.Errorf("want apiVersion forgelab/v1 and kind LearningPath, got %q %q", p.APIVersion, p.Kind)
	}
	if len(p.Spec.Stages) == 0 {
		return fmt.Errorf("at least one stage is required")
	}
	stages, exercises := map[string]bool{}, map[string]bool{}
	for _, s := range p.Spec.Stages {
		if s.ID == "" || s.Title == "" || stages[s.ID] {
			return fmt.Errorf("stage ids must be unique and stages need a title (%q)", s.ID)
		}
		stages[s.ID] = true
		if len(s.Exercises) == 0 {
			return fmt.Errorf("stage %q has no exercises", s.ID)
		}
		for _, e := range s.Exercises {
			if e.ID == "" || e.Title == "" || exercises[e.ID] {
				return fmt.Errorf("exercise ids must be unique and exercises need a title (%q)", e.ID)
			}
			exercises[e.ID] = true
			switch e.Evidence.Type {
			case EvidenceManual, EvidenceBenchmark:
			case EvidenceExperiment:
				if e.Evidence.Name == "" {
					return fmt.Errorf("exercise %q: experiment evidence needs a name", e.ID)
				}
			default:
				return fmt.Errorf("exercise %q: unknown evidence type %q", e.ID, e.Evidence.Type)
			}
		}
	}
	return nil
}

// Completion records when and how an exercise was completed.
type Completion struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// Progress is a learner's completed exercises.
type Progress struct {
	Completed map[string]Completion `json:"completed"`
}

// LoadProgress reads a progress file; a missing file is empty progress.
func LoadProgress(file string) (Progress, error) {
	pr := Progress{Completed: map[string]Completion{}}
	raw, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return pr, nil
		}
		return pr, err
	}
	if err := json.Unmarshal(raw, &pr); err != nil {
		return Progress{Completed: map[string]Completion{}}, fmt.Errorf("parse progress %q: %w", file, err)
	}
	if pr.Completed == nil {
		pr.Completed = map[string]Completion{}
	}
	return pr, nil
}

// Save writes the progress atomically, creating the directory if needed.
func (pr Progress) Save(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

func (p *Path) find(id string) (Exercise, bool) {
	for _, s := range p.Spec.Stages {
		for _, e := range s.Exercises {
			if e.ID == id {
				return e, true
			}
		}
	}
	return Exercise{}, false
}

// Complete marks an exercise done. Completing twice keeps the first record.
// It returns whether the exercise was newly completed.
func (pr *Progress) Complete(p *Path, id, by string, at time.Time) (bool, error) {
	if _, ok := p.find(id); !ok {
		return false, fmt.Errorf("unknown exercise %q", id)
	}
	if pr.Completed == nil {
		pr.Completed = map[string]Completion{}
	}
	if _, done := pr.Completed[id]; done {
		return false, nil
	}
	pr.Completed[id] = Completion{At: at.UTC(), By: by}
	return true, nil
}

// ExercisesFor lists the exercise ids completed by the given evidence.
func (p *Path) ExercisesFor(evidenceType, name string) []string {
	var ids []string
	for _, s := range p.Spec.Stages {
		for _, e := range s.Exercises {
			if e.Evidence.Type == evidenceType && (evidenceType != EvidenceExperiment || e.Evidence.Name == name) {
				ids = append(ids, e.ID)
			}
		}
	}
	return ids
}

// Record completes every exercise tied to the evidence and returns the ids
// that were newly completed.
func (pr *Progress) Record(p *Path, evidenceType, name string, at time.Time) []string {
	var fresh []string
	by := evidenceType
	if name != "" {
		by += ":" + name
	}
	for _, id := range p.ExercisesFor(evidenceType, name) {
		if newly, err := pr.Complete(p, id, by, at); err == nil && newly {
			fresh = append(fresh, id)
		}
	}
	return fresh
}

// ExerciseStatus is an exercise with its completion state.
type ExerciseStatus struct {
	Exercise
	Done        bool       `json:"done"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CompletedBy string     `json:"completedBy,omitempty"`
}

// StageStatus summarizes a stage.
type StageStatus struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Goal      string           `json:"goal"`
	Done      int              `json:"done"`
	Total     int              `json:"total"`
	Complete  bool             `json:"complete"`
	Exercises []ExerciseStatus `json:"exercises"`
}

// Status is the learner's overall position.
type Status struct {
	Stages []StageStatus `json:"stages"`
	Done   int           `json:"done"`
	Total  int           `json:"total"`
	// Next is the first incomplete exercise in path order, empty when finished.
	Next string `json:"next"`
}

// Status combines the path with progress.
func (p *Path) Status(pr Progress) Status {
	var st Status
	for _, s := range p.Spec.Stages {
		ss := StageStatus{ID: s.ID, Title: s.Title, Goal: s.Goal, Total: len(s.Exercises)}
		for _, e := range s.Exercises {
			es := ExerciseStatus{Exercise: e}
			if c, ok := pr.Completed[e.ID]; ok {
				at := c.At
				es.Done, es.CompletedAt, es.CompletedBy = true, &at, c.By
				ss.Done++
			} else if st.Next == "" {
				st.Next = e.ID
			}
			ss.Exercises = append(ss.Exercises, es)
		}
		ss.Complete = ss.Done == ss.Total
		st.Done += ss.Done
		st.Total += ss.Total
		st.Stages = append(st.Stages, ss)
	}
	return st
}
