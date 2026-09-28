package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
	"github.com/samin-al-wasee/production-simulator/core/internal/chaos"
)

type fakeExec struct {
	mu  sync.Mutex
	ran []string
	out string
	err error
}

func (f *fakeExec) Run(_ context.Context, cmd chaos.Command) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ran = append(f.ran, strings.Join(cmd, " "))
	return f.out, f.err
}

// fakeProber reports healthy unless the fault is active.
type fakeProber struct{ exec *fakeExec }

func (p fakeProber) Probe(_ context.Context, _ string) (chaos.Response, error) {
	p.exec.mu.Lock()
	defer p.exec.mu.Unlock()
	status := 200
	for _, r := range p.exec.ran {
		if strings.Contains(r, "docker stop") {
			status = 503
		}
		if strings.Contains(r, "docker start") {
			status = 200
		}
	}
	return chaos.Response{Status: status}, nil
}

const experimentYAML = `
apiVersion: forgelab/v1
kind: Experiment
metadata:
  name: db-outage
spec:
  hypothesis: degrades then recovers
  target:
    container: forgelab-db
  fault:
    type: stop
    duration: 5s
  probes:
    - {name: up, url: "http://x/healthz", phase: before, expectStatus: 200}
    - {name: down, url: "http://x/healthz", phase: during, expectStatus: 503}
    - {name: back, url: "http://x/healthz", phase: after, expectStatus: 200}
`

const clusterYAML = `
apiVersion: forgelab/v1
kind: Cluster
name: test
nodes:
  - {name: web, count: 10, profile: large}
`

const pipelineYAML = `
apiVersion: forgelab/v1
kind: Pipeline
metadata: {name: demo}
spec:
  stages:
    - name: build
      steps: [{name: compile, duration: 30s}]
    - name: deploy
      deploy: {strategy: canary, canarySteps: [50, 100], analysisDuration: 60s}
`

func newTestServer(t *testing.T, enableRuns bool) (*Server, *fakeExec) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("scenarios/failures/db-outage/experiment.yaml", experimentYAML)
	write("scenarios/failures/broken/experiment.yaml", "not: valid")
	write("manifests/cluster.example.yaml", clusterYAML)
	write("manifests/pipelines/demo.yaml", pipelineYAML)

	ex := &fakeExec{}
	s := NewServer(Config{RepoRoot: root, Reserve: 0.25, EnableRuns: enableRuns})
	s.Detect = func(string) (calibration.Host, error) {
		return calibration.Host{CPUCores: 8, MemoryBytes: 16 << 30, DiskBytes: 500 << 30}, nil
	}
	s.Usage = func(calibration.Host, string) (calibration.Host, error) {
		return calibration.Host{CPUCores: 2, MemoryBytes: 4 << 30, DiskBytes: 100 << 30}, nil
	}
	s.Exec = ex
	s.Probe = fakeProber{exec: ex}
	s.Sleep = func(time.Duration) {}
	return s, ex
}

func do(t *testing.T, h http.Handler, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var decoded map[string]any
	json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec, decoded
}

func TestHealthAndConfig(t *testing.T) {
	s, _ := newTestServer(t, false)
	if rec, _ := do(t, s, "GET", "/healthz", ""); rec.Code != 200 {
		t.Fatalf("healthz = %d", rec.Code)
	}
	_, cfg := do(t, s, "GET", "/api/v1/config", "")
	if cfg["runsEnabled"] != false {
		t.Fatalf("config = %v", cfg)
	}
}

func TestHostBudget(t *testing.T) {
	s, _ := newTestServer(t, false)
	_, body := do(t, s, "GET", "/api/v1/host", "")
	alloc := body["allocatable"].(map[string]any)
	if alloc["cpuCores"] != float64(6) {
		t.Fatalf("allocatable = %v", alloc)
	}
}

func TestClusterDualMetrics(t *testing.T) {
	s, _ := newTestServer(t, false)
	rec, body := do(t, s, "GET", "/api/v1/cluster?rps=100", "")
	if rec.Code != 200 {
		t.Fatalf("code = %d body = %s", rec.Code, rec.Body)
	}
	m := body["metrics"].(map[string]any)
	if m["scaleFactor"] == "" || m["scaleFactor"] == nil {
		t.Fatalf("scale factor must always be present: %v", m)
	}
	virt := m["virtual"].(map[string]any)
	phys := m["physical"].(map[string]any)
	if virt["kind"] != "virtual-production-simulated" || phys["kind"] != "physical-host-measurement" {
		t.Fatalf("views must be labelled: %v %v", phys["kind"], virt["kind"])
	}
	if rec, _ := do(t, s, "GET", "/api/v1/cluster?rps=abc", ""); rec.Code != 400 {
		t.Fatalf("bad rps code = %d", rec.Code)
	}
}

func TestStack(t *testing.T) {
	s, ex := newTestServer(t, false)
	ex.out = `{"Names":"forgelab-db","Image":"postgres","State":"running","Status":"Up 2 minutes"}` + "\n" +
		`{"Names":"forgelab-app","Image":"app","State":"exited","Status":"Exited (1)"}` + "\n"
	_, body := do(t, s, "GET", "/api/v1/stack", "")
	cs := body["containers"].([]any)
	if len(cs) != 2 || cs[0].(map[string]any)["name"] != "forgelab-app" {
		t.Fatalf("containers = %v", cs)
	}
}

type errDocker struct{}

func (errDocker) Error() string { return "docker not found" }

func TestStackWithoutDocker(t *testing.T) {
	s, ex := newTestServer(t, false)
	ex.err = errDocker{}
	ex.out = "docker: command not found"
	rec, body := do(t, s, "GET", "/api/v1/stack", "")
	if rec.Code != 200 || body["error"] == nil {
		t.Fatalf("code=%d body=%v", rec.Code, body)
	}
}

func TestExperimentsListSkipsInvalid(t *testing.T) {
	s, _ := newTestServer(t, false)
	req := httptest.NewRequest("GET", "/api/v1/experiments", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "db-outage" || list[0]["target"] != "forgelab-db" {
		t.Fatalf("list = %v", list)
	}
}

func TestRunsDisabledByDefault(t *testing.T) {
	s, ex := newTestServer(t, false)
	rec, _ := do(t, s, "POST", "/api/v1/experiments/db-outage/runs", "")
	if rec.Code != http.StatusForbidden || len(ex.ran) != 0 {
		t.Fatalf("code = %d ran = %v", rec.Code, ex.ran)
	}
}

func TestRunLifecycle(t *testing.T) {
	s, _ := newTestServer(t, true)
	if rec, _ := do(t, s, "POST", "/api/v1/experiments/nope/runs", ""); rec.Code != 404 {
		t.Fatalf("unknown experiment code = %d", rec.Code)
	}
	rec, body := do(t, s, "POST", "/api/v1/experiments/db-outage/runs", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code = %d body = %s", rec.Code, rec.Body)
	}
	id := body["id"].(string)

	var run map[string]any
	for i := 0; i < 100; i++ {
		_, run = do(t, s, "GET", "/api/v1/experiment-runs/"+id, "")
		if run["status"] != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run["status"] != "passed" {
		t.Fatalf("run = %v", run)
	}
	if !strings.Contains(run["log"].(string), "PASS") || run["report"].(map[string]any)["reverted"] != true {
		t.Fatalf("run = %v", run)
	}

	req := httptest.NewRequest("GET", "/api/v1/experiment-runs", nil)
	list := httptest.NewRecorder()
	s.ServeHTTP(list, req)
	var runs []map[string]any
	json.Unmarshal(list.Body.Bytes(), &runs)
	if len(runs) != 1 || runs[0]["log"] != "" {
		t.Fatalf("list must omit logs: %v", runs)
	}
	if rec, _ := do(t, s, "GET", "/api/v1/experiment-runs/missing", ""); rec.Code != 404 {
		t.Fatalf("missing run code = %d", rec.Code)
	}
}

func TestOnlyOneRunAtATime(t *testing.T) {
	s, _ := newTestServer(t, true)
	block := make(chan struct{})
	s.Sleep = func(time.Duration) { <-block }
	rec, _ := do(t, s, "POST", "/api/v1/experiments/db-outage/runs", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first run code = %d", rec.Code)
	}
	rec, _ = do(t, s, "POST", "/api/v1/experiments/db-outage/runs", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("second concurrent run code = %d", rec.Code)
	}
	close(block)
	for i := 0; i < 100; i++ {
		s.mu.Lock()
		running := false
		for _, r := range s.runs {
			running = running || r.Status == "running"
		}
		s.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("run never finished")
}

func TestPipelines(t *testing.T) {
	s, _ := newTestServer(t, false)
	req := httptest.NewRequest("GET", "/api/v1/pipelines", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "demo" || len(list[0]["stages"].([]any)) != 2 {
		t.Fatalf("list = %v", list)
	}

	_, run := do(t, s, "POST", "/api/v1/pipelines/demo/runs", "")
	if run["status"] != "succeeded" {
		t.Fatalf("run = %v", run)
	}
	_, run = do(t, s, "POST", "/api/v1/pipelines/demo/runs", `{"badRelease": true}`)
	if run["status"] != "rolled-back" {
		t.Fatalf("bad release run = %v", run)
	}
	if rec, _ := do(t, s, "POST", "/api/v1/pipelines/demo/runs", `{"nope": 1}`); rec.Code != 400 {
		t.Fatalf("unknown field code = %d", rec.Code)
	}
	if rec, _ := do(t, s, "POST", "/api/v1/pipelines/missing/runs", ""); rec.Code != 404 {
		t.Fatalf("missing pipeline code = %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	s, _ := newTestServer(t, false)
	s.cfg.AllowedOrigin = "http://localhost:3001"
	req := httptest.NewRequest("OPTIONS", "/api/v1/host", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3001" {
		t.Fatalf("code=%d headers=%v", rec.Code, rec.Header())
	}
	rec2 := httptest.NewRecorder()
	newRec, _ := newTestServer(t, false)
	newRec.ServeHTTP(rec2, httptest.NewRequest("GET", "/healthz", nil))
	if rec2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS header must be absent when no origin is configured")
	}
}
