// Package api exposes the simulation core over HTTP for the dashboard. It is
// a thin adapter: every response is computed by the core packages, and the
// dashboard never duplicates that logic. Handlers that change the world
// (starting chaos experiments) are disabled unless explicitly enabled.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
	"github.com/samin-al-wasee/production-simulator/core/internal/chaos"
	"github.com/samin-al-wasee/production-simulator/core/internal/learning"
	"github.com/samin-al-wasee/production-simulator/core/internal/metrics"
	"github.com/samin-al-wasee/production-simulator/core/internal/pipeline"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
	"github.com/samin-al-wasee/production-simulator/core/internal/virtualcluster"
)

// Config configures a Server.
type Config struct {
	// RepoRoot is the repository root, used to find scenarios/ and manifests/.
	RepoRoot string
	// ClusterFile is the virtual cluster spec, relative to RepoRoot.
	ClusterFile string
	// DiskPath selects the filesystem reported by host detection.
	DiskPath string
	// Reserve is the host reserve fraction in [0, 1).
	Reserve float64
	// LearningPath is the learning path file, relative to RepoRoot.
	LearningPath string
	// ProgressFile stores learner progress; default .forgelab/progress.json
	// under RepoRoot.
	ProgressFile string
	// EnableRuns allows starting chaos experiments through the API.
	EnableRuns bool
	// AllowedOrigin, when set, is returned in CORS headers.
	AllowedOrigin string
}

// Server implements http.Handler.
type Server struct {
	cfg Config

	// Collaborators, replaceable in tests.
	Detect func(diskPath string) (calibration.Host, error)
	Usage  func(host calibration.Host, diskPath string) (calibration.Host, error)
	Exec   chaos.Executor
	Probe  chaos.Prober
	Sleep  func(time.Duration)

	progressMu sync.Mutex

	mux  *http.ServeMux
	mu   sync.Mutex
	runs map[string]*ExperimentRun
	seq  int

	games   map[string]*sandboxGame
	gameSeq int
}

// NewServer builds a Server using the real host, Docker, and HTTP.
func NewServer(cfg Config) *Server {
	if cfg.ClusterFile == "" {
		cfg.ClusterFile = "manifests/cluster.example.yaml"
	}
	if cfg.DiskPath == "" {
		cfg.DiskPath = "."
	}
	if cfg.LearningPath == "" {
		cfg.LearningPath = "learning/path.yaml"
	}
	if cfg.ProgressFile == "" {
		cfg.ProgressFile = filepath.Join(cfg.RepoRoot, ".forgelab", "progress.json")
	}
	s := &Server{
		cfg:    cfg,
		Detect: calibration.Detect,
		Usage:  calibration.Usage,
		Exec:   chaos.DockerExecutor{},
		Probe:  chaos.HTTPProber{},
		Sleep:  time.Sleep,
		runs:   map[string]*ExperimentRun{},
		games:  map[string]*sandboxGame{},
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/v1/host", s.handleHost)
	s.mux.HandleFunc("GET /api/v1/cluster", s.handleCluster)
	s.mux.HandleFunc("GET /api/v1/stack", s.handleStack)
	s.mux.HandleFunc("GET /api/v1/experiments", s.handleExperiments)
	s.mux.HandleFunc("POST /api/v1/experiments/{name}/runs", s.handleStartRun)
	s.mux.HandleFunc("GET /api/v1/experiment-runs", s.handleListRuns)
	s.mux.HandleFunc("GET /api/v1/experiment-runs/{id}", s.handleGetRun)
	s.mux.HandleFunc("GET /api/v1/learning", s.handleLearning)
	s.mux.HandleFunc("POST /api/v1/learning/{id}/complete", s.handleLearningComplete)
	s.mux.HandleFunc("GET /api/v1/pipelines", s.handlePipelines)
	s.mux.HandleFunc("POST /api/v1/pipelines/{name}/runs", s.handleSimulatePipeline)
	s.registerSandbox()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AllowedOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, format string, a ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, a...)})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runsEnabled": s.cfg.EnableRuns, "clusterFile": s.cfg.ClusterFile})
}

func (s *Server) budget() (budget.Budget, calibration.Host, error) {
	host, err := s.Detect(s.cfg.DiskPath)
	if err != nil {
		return budget.Budget{}, calibration.Host{}, err
	}
	b, err := budget.Compute(host, budget.UniformPolicy(s.cfg.Reserve))
	return b, host, err
}

func (s *Server) handleHost(w http.ResponseWriter, _ *http.Request) {
	b, _, err := s.budget()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	cluster, err := virtualcluster.Load(filepath.Join(s.cfg.RepoRoot, s.cfg.ClusterFile))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	b, host, err := s.budget()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	factor, err := scale.Compute(b.Allocatable, cluster.Total())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	used, err := s.Usage(host, s.cfg.DiskPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	var rps float64
	if v := r.URL.Query().Get("rps"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &rps); err != nil || rps < 0 {
			writeError(w, http.StatusBadRequest, "rps must be a non-negative number")
			return
		}
	}
	dual := metrics.Translate(factor, b.Allocatable,
		budget.Resources{CPUCores: used.CPUCores, MemoryBytes: used.MemoryBytes, DiskBytes: used.DiskBytes},
		rps, cluster.Total(), cluster.Nodes())
	writeJSON(w, http.StatusOK, map[string]any{
		"cluster": cluster.Name,
		"factor":  factor,
		"metrics": dual,
	})
}

// Container is one row of the local stack status.
type Container struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
}

func (s *Server) handleStack(w http.ResponseWriter, r *http.Request) {
	out, err := s.Exec.Run(r.Context(), chaos.Command{
		"docker", "ps", "-a", "--filter", "label=com.docker.compose.project=forgelab-local",
		"--format", "{{json .}}",
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []Container{}, "error": "docker unavailable: " + strings.TrimSpace(firstLine(out))})
		return
	}
	containers := []Container{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var row struct {
			Names  string `json:"Names"`
			Image  string `json:"Image"`
			State  string `json:"State"`
			Status string `json:"Status"`
		}
		if json.Unmarshal([]byte(line), &row) == nil {
			containers = append(containers, Container{row.Names, row.Image, row.State, row.Status})
		}
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"containers": containers})
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// ExperimentInfo describes a discoverable experiment.
type ExperimentInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Hypothesis string `json:"hypothesis"`
	Target     string `json:"target"`
	Fault      string `json:"fault"`
	Duration   string `json:"duration"`
}

func (s *Server) discoverExperiments() (map[string]*chaos.Experiment, []ExperimentInfo, error) {
	root := filepath.Join(s.cfg.RepoRoot, "scenarios")
	found := map[string]*chaos.Experiment{}
	var infos []ExperimentInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "experiment.yaml" {
			return nil
		}
		e, err := chaos.Load(path)
		if err != nil {
			return nil // invalid experiments are not offered
		}
		rel, _ := filepath.Rel(s.cfg.RepoRoot, path)
		found[e.Metadata.Name] = e
		infos = append(infos, ExperimentInfo{
			Name: e.Metadata.Name, Path: rel, Hypothesis: e.Spec.Hypothesis,
			Target: e.Spec.Target.Container, Fault: e.Spec.Fault.Type,
			Duration: time.Duration(e.Spec.Fault.Duration).String(),
		})
		return nil
	})
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return found, infos, err
}

func (s *Server) handleExperiments(w http.ResponseWriter, _ *http.Request) {
	_, infos, err := s.discoverExperiments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if infos == nil {
		infos = []ExperimentInfo{}
	}
	writeJSON(w, http.StatusOK, infos)
}

// ExperimentRun is the state of one launched experiment.
type ExperimentRun struct {
	ID         string        `json:"id"`
	Experiment string        `json:"experiment"`
	Status     string        `json:"status"` // running, passed, failed, error
	Log        string        `json:"log"`
	Report     *chaos.Report `json:"report,omitempty"`
	Error      string        `json:"error,omitempty"`
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt *time.Time    `json:"finishedAt,omitempty"`
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.EnableRuns {
		writeError(w, http.StatusForbidden, "starting experiments is disabled; start the server with -enable-runs")
		return
	}
	experiments, _, err := s.discoverExperiments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	e, ok := experiments[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown experiment %q", r.PathValue("name"))
		return
	}

	s.mu.Lock()
	for _, existing := range s.runs {
		if existing.Status == "running" {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "experiment run %s is still in progress", existing.ID)
			return
		}
	}
	s.seq++
	run := &ExperimentRun{ID: fmt.Sprintf("run-%d", s.seq), Experiment: e.Metadata.Name, Status: "running", StartedAt: time.Now().UTC()}
	s.runs[run.ID] = run
	s.mu.Unlock()

	log := &lockedBuffer{}
	go func() {
		// The request context ends when the response is sent; the run must not.
		rep, err := chaos.Run(context.Background(), e, chaos.Env{Exec: s.Exec, Probe: s.Probe, Sleep: s.Sleep, Log: log})
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now().UTC()
		run.FinishedAt = &now
		run.Report = &rep
		switch {
		case err != nil:
			run.Status, run.Error = "error", err.Error()
		case rep.Passed:
			run.Status = "passed"
			s.recordExperiment(e.Metadata.Name)
		default:
			run.Status = "failed"
		}
		run.Log = log.String()
	}()
	s.liveLogs(run.ID, log)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": run.ID})
}

// liveLogs mirrors a running experiment's log into its run record so polling
// clients see progress before completion.
func (s *Server) liveLogs(id string, log *lockedBuffer) {
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			s.mu.Lock()
			run := s.runs[id]
			done := run == nil || run.Status != "running"
			if !done {
				run.Log = log.String()
			}
			s.mu.Unlock()
			if done {
				return
			}
		}
	}()
}

func (s *Server) handleListRuns(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := make([]ExperimentRun, 0, len(s.runs))
	for _, r := range s.runs {
		c := *r
		c.Log = ""
		out = append(out, c)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	run, ok := s.runs[r.PathValue("id")]
	var c ExperimentRun
	if ok {
		c = *run
	}
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "unknown run %q", r.PathValue("id"))
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// PipelineInfo describes a discoverable pipeline.
type PipelineInfo struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Stages []string `json:"stages"`
}

func (s *Server) discoverPipelines() (map[string]*pipeline.Pipeline, []PipelineInfo, error) {
	dir := filepath.Join(s.cfg.RepoRoot, "manifests", "pipelines")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*pipeline.Pipeline{}, []PipelineInfo{}, nil
		}
		return nil, nil, err
	}
	found := map[string]*pipeline.Pipeline{}
	infos := []PipelineInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		p, err := pipeline.Load(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var stages []string
		for _, st := range p.Spec.Stages {
			stages = append(stages, st.Name)
		}
		found[p.Metadata.Name] = p
		infos = append(infos, PipelineInfo{Name: p.Metadata.Name, Path: filepath.Join("manifests", "pipelines", e.Name()), Stages: stages})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return found, infos, nil
}

func (s *Server) handlePipelines(w http.ResponseWriter, _ *http.Request) {
	_, infos, err := s.discoverPipelines()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, infos)
}

func (s *Server) handleSimulatePipeline(w http.ResponseWriter, r *http.Request) {
	pipelines, _, err := s.discoverPipelines()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	p, ok := pipelines[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown pipeline %q", r.PathValue("name"))
		return
	}
	var opt pipeline.Options
	opt.Seed = 1
	if r.ContentLength != 0 {
		var body struct {
			Seed       *int64 `json:"seed"`
			WarmCache  bool   `json:"warmCache"`
			FailStep   string `json:"failStep"`
			BadRelease bool   `json:"badRelease"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: %v", err)
			return
		}
		if body.Seed != nil {
			opt.Seed = *body.Seed
		}
		opt.WarmCache, opt.FailStep, opt.BadRelease = body.WarmCache, body.FailStep, body.BadRelease
	}
	run, err := pipeline.Simulate(p, opt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) loadLearning() (*learning.Path, learning.Progress, error) {
	path, err := learning.LoadPath(filepath.Join(s.cfg.RepoRoot, s.cfg.LearningPath))
	if err != nil {
		return nil, learning.Progress{}, err
	}
	progress, err := learning.LoadProgress(s.cfg.ProgressFile)
	return path, progress, err
}

// recordExperiment completes the learning exercises tied to a passing
// experiment. Problems are ignored: a run's result must not depend on
// bookkeeping.
func (s *Server) recordExperiment(name string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	path, progress, err := s.loadLearning()
	if err != nil {
		return
	}
	if fresh := progress.Record(path, learning.EvidenceExperiment, name, time.Now()); len(fresh) > 0 {
		progress.Save(s.cfg.ProgressFile)
	}
}

func (s *Server) handleLearning(w http.ResponseWriter, _ *http.Request) {
	s.progressMu.Lock()
	path, progress, err := s.loadLearning()
	s.progressMu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, path.Status(progress))
}

func (s *Server) handleLearningComplete(w http.ResponseWriter, r *http.Request) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	path, progress, err := s.loadLearning()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err := progress.Complete(path, r.PathValue("id"), "manual", time.Now()); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := progress.Save(s.cfg.ProgressFile); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, path.Status(progress))
}
