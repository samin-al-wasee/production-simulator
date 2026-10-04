// Package api exposes the simulation core over HTTP for the dashboard. It is
// a thin adapter: every response is computed by the core packages (the
// Sandbox engine, the pipeline simulator, and the learning tracker), and the
// dashboard never duplicates that logic.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/identity"
	"github.com/samin-al-wasee/production-simulator/backend/internal/learning"
	"github.com/samin-al-wasee/production-simulator/backend/internal/pipeline"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

// Config configures a Server.
type Config struct {
	// RepoRoot is the repository root, used to find learning/ and manifests/.
	RepoRoot string
	// LearningPath is the learning path file, relative to RepoRoot.
	LearningPath string
	// ProgressFile stores learner progress; default .forgelab/progress.json
	// under RepoRoot.
	ProgressFile string
	// AllowedOrigin, when set, is returned in CORS headers.
	AllowedOrigin string
	// Store is the application database. When nil the API runs without one,
	// exactly as before the application layer existed (ADR-0030).
	Store *store.Store
	// Identity runs OAuth sign-in and sessions. When nil the auth endpoints
	// report that sign-in is not configured (ADR-0031).
	Identity *identity.Service
}

// Server implements http.Handler.
type Server struct {
	cfg Config

	progressMu sync.Mutex

	mux *http.ServeMux
	mu  sync.Mutex

	games   map[string]*sandboxGame
	gameSeq int
}

// NewServer builds a Server.
func NewServer(cfg Config) *Server {
	if cfg.LearningPath == "" {
		cfg.LearningPath = "learning/path.yaml"
	}
	if cfg.ProgressFile == "" {
		cfg.ProgressFile = filepath.Join(cfg.RepoRoot, ".forgelab", "progress.json")
	}
	s := &Server{cfg: cfg, games: map[string]*sandboxGame{}}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("GET /api/v1/learning", s.handleLearning)
	s.mux.HandleFunc("POST /api/v1/learning/{id}/complete", s.handleLearningComplete)
	s.mux.HandleFunc("GET /api/v1/pipelines", s.handlePipelines)
	s.mux.HandleFunc("POST /api/v1/pipelines/{name}/runs", s.handleSimulatePipeline)
	s.registerSandbox()
	s.registerAuth()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AllowedOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
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

// handleReady reports readiness: the API is up and, when configured, the
// application store is reachable.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "none"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.cfg.Store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unreachable: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
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
