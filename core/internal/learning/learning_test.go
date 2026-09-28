package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testPath = `
apiVersion: forgelab/v1
kind: LearningPath
metadata: {name: t}
spec:
  stages:
    - id: a
      title: A
      exercises:
        - {id: a1, title: one, evidence: {type: manual}}
        - {id: a2, title: two, evidence: {type: experiment, name: db-outage}}
    - id: b
      title: B
      exercises:
        - {id: b1, title: three, evidence: {type: experiment, name: db-outage}}
        - {id: b2, title: four, evidence: {type: benchmark}}
`

func load(t *testing.T, doc string) *Path {
	t.Helper()
	f := filepath.Join(t.TempDir(), "path.yaml")
	os.WriteFile(f, []byte(doc), 0o644)
	p, err := LoadPath(f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var now = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func TestStatusAndNext(t *testing.T) {
	p := load(t, testPath)
	st := p.Status(Progress{})
	if st.Total != 4 || st.Done != 0 || st.Next != "a1" || st.Stages[0].Complete {
		t.Fatalf("status = %+v", st)
	}
	pr := Progress{}
	pr.Complete(p, "a1", "manual", now)
	pr.Complete(p, "a2", "manual", now)
	st = p.Status(pr)
	if st.Done != 2 || st.Next != "b1" || !st.Stages[0].Complete || st.Stages[1].Complete {
		t.Fatalf("status = %+v", st)
	}
	if st.Stages[0].Exercises[0].CompletedBy != "manual" || st.Stages[0].Exercises[0].CompletedAt == nil {
		t.Fatalf("completion details missing: %+v", st.Stages[0].Exercises[0])
	}
}

func TestCompleteIsIdempotentAndValidates(t *testing.T) {
	p := load(t, testPath)
	var pr Progress
	if fresh, err := pr.Complete(p, "a1", "manual", now); err != nil || !fresh {
		t.Fatalf("first: %v %v", fresh, err)
	}
	later := now.Add(time.Hour)
	if fresh, _ := pr.Complete(p, "a1", "other", later); fresh {
		t.Fatal("second completion must not be fresh")
	}
	if c := pr.Completed["a1"]; !c.At.Equal(now) || c.By != "manual" {
		t.Fatalf("first record must be kept: %+v", c)
	}
	if _, err := pr.Complete(p, "nope", "manual", now); err == nil {
		t.Fatal("expected error for unknown exercise")
	}
}

func TestRecordCompletesMatchingEvidenceOnly(t *testing.T) {
	p := load(t, testPath)
	var pr Progress
	fresh := pr.Record(p, EvidenceExperiment, "db-outage", now)
	if strings.Join(fresh, ",") != "a2,b1" {
		t.Fatalf("fresh = %v", fresh)
	}
	if got := pr.Record(p, EvidenceExperiment, "db-outage", now); len(got) != 0 {
		t.Fatalf("already recorded, got %v", got)
	}
	if got := pr.Record(p, EvidenceExperiment, "redis-outage", now); len(got) != 0 {
		t.Fatalf("other experiment must not complete anything: %v", got)
	}
	if got := pr.Record(p, EvidenceBenchmark, "", now); len(got) != 1 || got[0] != "b2" {
		t.Fatalf("benchmark record = %v", got)
	}
	if pr.Completed["a2"].By != "experiment:db-outage" {
		t.Fatalf("by = %q", pr.Completed["a2"].By)
	}
}

func TestProgressRoundTrip(t *testing.T) {
	p := load(t, testPath)
	file := filepath.Join(t.TempDir(), "sub", "progress.json")
	pr, err := LoadProgress(file)
	if err != nil || len(pr.Completed) != 0 {
		t.Fatalf("missing file must be empty: %v %v", pr, err)
	}
	pr.Complete(p, "a1", "manual", now)
	if err := pr.Save(file); err != nil {
		t.Fatal(err)
	}
	back, err := LoadProgress(file)
	if err != nil || !back.Completed["a1"].At.Equal(now) {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	os.WriteFile(file, []byte("{not json"), 0o644)
	if _, err := LoadProgress(file); err == nil {
		t.Fatal("expected error for corrupt progress")
	}
}

func TestValidate(t *testing.T) {
	bad := map[string]string{
		"wrong kind":  strings.Replace(testPath, "LearningPath", "X", 1),
		"no stages":   "apiVersion: forgelab/v1\nkind: LearningPath\nmetadata: {name: t}\nspec: {stages: []}\n",
		"dup stage":   strings.Replace(testPath, "id: b\n", "id: a\n", 1),
		"dup ex":      strings.Replace(testPath, "id: b1", "id: a1", 1),
		"no name":     strings.Replace(testPath, "evidence: {type: experiment, name: db-outage}}\n    - id: b", "evidence: {type: experiment}}\n    - id: b", 1),
		"bad type":    strings.Replace(testPath, "type: benchmark", "type: vibes", 1),
		"unknown key": strings.Replace(testPath, "metadata: {name: t}", "metadata: {name: t}\nbogus: 1", 1),
	}
	for name, doc := range bad {
		f := filepath.Join(t.TempDir(), "p.yaml")
		os.WriteFile(f, []byte(doc), 0o644)
		if _, err := LoadPath(f); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// The repository's own path must stay valid and reference real experiments.
func TestRepositoryPath(t *testing.T) {
	p, err := LoadPath("../../../learning/path.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Spec.Stages {
		for _, e := range s.Exercises {
			if e.Evidence.Type != EvidenceExperiment {
				continue
			}
			if _, err := os.Stat("../../../scenarios/failures/" + e.Evidence.Name + "/experiment.yaml"); err != nil {
				t.Errorf("exercise %s references missing experiment %q", e.ID, e.Evidence.Name)
			}
		}
	}
	if len(p.Spec.Stages) != 10 {
		t.Errorf("stages = %d, want 10 (docs/learning-path.md)", len(p.Spec.Stages))
	}
}
