package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

const learningYAML = `
apiVersion: forgelab/v1
kind: LearningPath
metadata: {name: t}
spec:
  stages:
    - id: s1
      title: One
      exercises:
        - {id: s1-a, title: first, evidence: {type: manual}}
        - {id: s1-b, title: second, evidence: {type: manual}}
`

func newTestServer(t *testing.T) *Server {
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
	write("manifests/pipelines/demo.yaml", pipelineYAML)
	write("learning/path.yaml", learningYAML)
	return NewServer(Config{RepoRoot: root})
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

func TestHealth(t *testing.T) {
	rec, body := do(t, newTestServer(t), "GET", "/healthz", "")
	if rec.Code != 200 || body["status"] != "ok" {
		t.Fatalf("code=%d body=%v", rec.Code, body)
	}
}

func TestReadyWithoutStore(t *testing.T) {
	rec, body := do(t, newTestServer(t), "GET", "/readyz", "")
	if rec.Code != 200 || body["status"] != "ok" || body["database"] != "none" {
		t.Fatalf("code=%d body=%v", rec.Code, body)
	}
}

func TestPipelines(t *testing.T) {
	s := newTestServer(t)
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
	s := newTestServer(t)
	s.cfg.AllowedOrigin = "http://localhost:3001"
	req := httptest.NewRequest("OPTIONS", "/api/v1/pipelines", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3001" {
		t.Fatalf("code=%d headers=%v", rec.Code, rec.Header())
	}
	rec2 := httptest.NewRecorder()
	newRec := newTestServer(t)
	newRec.ServeHTTP(rec2, httptest.NewRequest("GET", "/healthz", nil))
	if rec2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS header must be absent when no origin is configured")
	}
}

func TestLearningStatusAndManualCompletion(t *testing.T) {
	s := newTestServer(t)
	_, st := do(t, s, "GET", "/api/v1/learning", "")
	if st["total"] != float64(2) || st["done"] != float64(0) || st["next"] != "s1-a" {
		t.Fatalf("status = %v", st)
	}
	rec, st := do(t, s, "POST", "/api/v1/learning/s1-a/complete", "")
	if rec.Code != 200 || st["done"] != float64(1) || st["next"] != "s1-b" {
		t.Fatalf("code=%d status=%v", rec.Code, st)
	}
	if rec, _ := do(t, s, "POST", "/api/v1/learning/nope/complete", ""); rec.Code != 404 {
		t.Fatalf("unknown exercise code = %d", rec.Code)
	}
	if _, err := os.Stat(s.cfg.ProgressFile); err != nil {
		t.Fatalf("progress must be persisted: %v", err)
	}
}
