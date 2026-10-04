package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/learning"
	"github.com/samin-al-wasee/production-simulator/backend/internal/sandbox"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

// Sandbox games live in memory; each is driven by its own clock goroutine
// while its speed is above zero. When the limit is reached the oldest game is
// dropped. Every value comes from the sandbox engine.

const maxSandboxGames = 20

// sandboxSpeeds are the allowed ticks per real second; 0 pauses.
var sandboxSpeeds = map[int]bool{0: true, 1: true, 2: true, 4: true, 8: true}

type sandboxGame struct {
	id  string
	seq int
	// userID is the owner, empty for an anonymous game (ADR-0031).
	userID string
	// saveID is the store row this game is saved to, empty until first saved.
	saveID string

	mu    sync.Mutex
	game  *sandbox.Game
	speed int
	// rev increases with every published change so clients can drop a
	// snapshot that arrives after a newer one.
	rev  int
	stop chan struct{}
	subs map[chan []byte]bool
	// recorded are the goals already passed to record, which completes the
	// learning-path exercises tied to them.
	recorded map[string]bool
	record   func(userID, game string, goals []string) error
}

// SandboxState is the full game view sent to the dashboard.
type SandboxState struct {
	ID        string           `json:"id"`
	Simulated bool             `json:"simulated"`
	Ruleset   string           `json:"ruleset"`
	Seed      int64            `json:"seed"`
	FreeBuild bool             `json:"freeBuild,omitempty"`
	Status    string           `json:"status"`
	Speed     int              `json:"speed"`
	Revision  int              `json:"revision"`
	Tick      int              `json:"tick"`
	Meters    sandbox.Meters   `json:"meters"`
	Nodes     []*sandbox.Node  `json:"nodes"`
	Edges     []sandbox.Edge   `json:"edges"`
	Flow      sandbox.Flow     `json:"flow"`
	History   []sandbox.Meters `json:"history"`
	// Events are upcoming, active, and recently judged events, oldest first.
	Events []sandbox.Event `json:"events"`
	// Goals are the ruleset's goals with their progress.
	Goals []sandbox.GoalStatus `json:"goals"`
	// Report explains the last tick, and NodeHistory holds each monitored
	// component's metrics (v14 and later).
	Report      *sandbox.Report                 `json:"report,omitempty"`
	NodeHistory map[string][]sandbox.NodeSample `json:"nodeHistory,omitempty"`
	// Monitoring is the alert rules and SLOs; Alerts where each rule
	// stands, AlertLog what fired, SLOs each SLO measured, and Timeline
	// events, alerts, and commands in order (v14).
	Monitoring *sandbox.Monitoring     `json:"monitoring,omitempty"`
	Alerts     []sandbox.AlertState    `json:"alerts,omitempty"`
	AlertLog   []sandbox.AlertEvent    `json:"alertLog,omitempty"`
	SLOs       []sandbox.SLOStatus     `json:"slos,omitempty"`
	Timeline   []sandbox.TimelineEntry `json:"timeline,omitempty"`
}

// state must be called with sg.mu held.
func (sg *sandboxGame) state() SandboxState {
	g := sg.game
	nodes := make([]*sandbox.Node, len(g.Nodes))
	for i, n := range g.Nodes {
		c := *n
		nodes[i] = &c
	}
	events := make([]sandbox.Event, len(g.Events))
	for i, e := range g.Events {
		events[i] = *e
		events[i].Hits = append([]sandbox.Hit(nil), e.Hits...)
	}
	return SandboxState{
		ID: sg.id, Simulated: true, Ruleset: g.Rules.Version, Seed: g.Seed, FreeBuild: g.FreeBuild,
		Status: g.Status, Speed: sg.speed, Revision: sg.rev, Tick: g.Tick,
		Meters: g.Last.Meters, Nodes: nodes, Edges: append([]sandbox.Edge{}, g.Edges...),
		Flow: g.Last.Flow, History: append([]sandbox.Meters{}, g.History...), Events: events,
		Goals: g.Goals(), Report: g.Report(), NodeHistory: g.NodeHistory(),
		Monitoring: g.Monitoring, Alerts: g.AlertStates(), AlertLog: append([]sandbox.AlertEvent(nil), g.AlertLog...),
		SLOs: g.SLOStatuses(), Timeline: g.Timeline(),
	}
}

// recordGoals must be called with sg.mu held. It passes newly reached goals
// to the learning path; progress never feeds back into the game. Goals that
// could not be recorded are offered again after the next change.
func (sg *sandboxGame) recordGoals() {
	var fresh []string
	for _, gl := range sg.game.Rules.Goals {
		if sg.game.Reached(gl.ID) && !sg.recorded[gl.ID] {
			fresh = append(fresh, gl.ID)
		}
	}
	if len(fresh) == 0 || sg.record == nil || sg.record(sg.userID, sg.id, fresh) != nil {
		return
	}
	for _, id := range fresh {
		sg.recorded[id] = true
	}
}

// publish must be called with sg.mu held, after every change. Slow
// subscribers miss intermediate states; they always receive a later one.
func (sg *sandboxGame) publish() {
	sg.recordGoals()
	sg.rev++
	if len(sg.subs) == 0 {
		return
	}
	b, _ := json.Marshal(sg.state())
	for ch := range sg.subs {
		select {
		case ch <- b:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- b
		}
	}
}

// setSpeed must be called with sg.mu held.
func (sg *sandboxGame) setSpeed(speed int) {
	if sg.stop != nil {
		close(sg.stop)
		sg.stop = nil
	}
	sg.speed = speed
	if speed == 0 || sg.game.Status != sandbox.StatusRunning {
		return
	}
	stop := make(chan struct{})
	sg.stop = stop
	go func() {
		t := time.NewTicker(time.Second / time.Duration(speed))
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				sg.mu.Lock()
				select {
				case <-stop:
					sg.mu.Unlock()
					return
				default:
				}
				sg.game.Step()
				if sg.game.Status != sandbox.StatusRunning {
					sg.setSpeed(0)
				}
				sg.publish()
				sg.mu.Unlock()
			}
		}
	}()
}

// end stops a game's clock and closes its streams once it is no longer held.
func (sg *sandboxGame) end() {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.setSpeed(0)
	for ch := range sg.subs {
		close(ch)
		delete(sg.subs, ch)
	}
}

func (s *Server) registerSandbox() {
	s.mux.HandleFunc("GET /api/v1/sandbox/ruleset", s.handleSandboxRuleset)
	s.mux.HandleFunc("GET /api/v1/sandbox/games", s.handleSandboxList)
	s.mux.HandleFunc("POST /api/v1/sandbox/games", s.handleSandboxCreate)
	s.mux.HandleFunc("GET /api/v1/sandbox/games/{id}", s.handleSandboxGet)
	s.mux.HandleFunc("DELETE /api/v1/sandbox/games/{id}", s.handleSandboxDelete)
	s.mux.HandleFunc("POST /api/v1/sandbox/games/{id}/commands", s.handleSandboxCommand)
	s.mux.HandleFunc("POST /api/v1/sandbox/games/{id}/speed", s.handleSandboxSpeed)
	s.mux.HandleFunc("POST /api/v1/sandbox/games/{id}/step", s.handleSandboxStep)
	s.mux.HandleFunc("POST /api/v1/sandbox/games/{id}/save", s.handleSandboxSave)
	s.mux.HandleFunc("GET /api/v1/sandbox/games/{id}/stream", s.handleSandboxStream)
	s.mux.HandleFunc("GET /api/v1/sandbox/saves", s.handleSandboxSavesList)
	s.mux.HandleFunc("GET /api/v1/sandbox/saves/{id}", s.handleSandboxSaveGet)
	s.mux.HandleFunc("DELETE /api/v1/sandbox/saves/{id}", s.handleSandboxSaveDelete)
}

// sandboxGame finds a game the caller owns. Another user's game is a 404, so
// one player cannot see or touch another's (ADR-0031).
func (s *Server) sandboxGame(w http.ResponseWriter, r *http.Request) *sandboxGame {
	user, _ := s.currentUser(r)
	s.mu.Lock()
	sg := s.games[r.PathValue("id")]
	s.mu.Unlock()
	if sg == nil || sg.userID != user.ID {
		writeError(w, http.StatusNotFound, "unknown game %q", r.PathValue("id"))
		return nil
	}
	return sg
}

// handleSandboxRuleset serves the latest ruleset, or the one a version names
// so an older game is shown with its own catalog and defaults.
func (s *Server) handleSandboxRuleset(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query().Get("version")
	if v == "" {
		writeJSON(w, http.StatusOK, sandbox.Latest())
		return
	}
	rules, err := sandbox.Rulesets(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleSandboxList(w http.ResponseWriter, r *http.Request) {
	type summary struct {
		ID     string  `json:"id"`
		Status string  `json:"status"`
		Tick   int     `json:"tick"`
		Users  float64 `json:"users"`
		Cash   float64 `json:"cash"`
	}
	user, _ := s.currentUser(r)
	s.mu.Lock()
	games := make([]*sandboxGame, 0, len(s.games))
	for _, sg := range s.games {
		if sg.userID == user.ID {
			games = append(games, sg)
		}
	}
	s.mu.Unlock()
	out := make([]summary, 0, len(games))
	for _, sg := range games {
		sg.mu.Lock()
		out = append(out, summary{ID: sg.id, Status: sg.game.Status, Tick: sg.game.Tick, Users: sg.game.Users, Cash: sg.game.Cash})
		sg.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

// handleSandboxCreate starts a new empty game, replays a save from the body,
// or resumes an owned saved sandbox named by saveId.
func (s *Server) handleSandboxCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seed *int64 `json:"seed"`
		// FreeBuild unlocks every kind from the start.
		FreeBuild bool          `json:"freeBuild"`
		Ruleset   string        `json:"ruleset"`
		Save      *sandbox.Save `json:"save"`
		// SaveID resumes an owned saved sandbox (signed-in players only).
		SaveID string `json:"saveId"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: %v", err)
			return
		}
	}
	user, _ := s.currentUser(r)
	var g *sandbox.Game
	var saveID string
	switch {
	case req.SaveID != "":
		if s.store == nil || user.ID == "" {
			writeError(w, http.StatusUnauthorized, "sign in to resume a saved sandbox")
			return
		}
		saved, err := s.store.GetSandbox(r.Context(), user.ID, req.SaveID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "unknown save %q", req.SaveID)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		var save sandbox.Save
		if err := json.Unmarshal([]byte(saved.Save), &save); err != nil {
			writeError(w, http.StatusInternalServerError, "bad save: %v", err)
			return
		}
		if g, err = sandbox.Replay(save); err != nil {
			writeError(w, http.StatusBadRequest, "replay save: %v", err)
			return
		}
		saveID = saved.ID
	case req.Save != nil:
		var err error
		if g, err = sandbox.Replay(*req.Save); err != nil {
			writeError(w, http.StatusBadRequest, "replay save: %v", err)
			return
		}
	default:
		seed := time.Now().UnixNano()
		if req.Seed != nil {
			seed = *req.Seed
		}
		rules := sandbox.Latest()
		if req.Ruleset != "" {
			var err error
			if rules, err = sandbox.Rulesets(req.Ruleset); err != nil {
				writeError(w, http.StatusBadRequest, "%v", err)
				return
			}
		}
		g = sandbox.New(rules, seed)
		g.FreeBuild = req.FreeBuild
	}

	s.mu.Lock()
	var evicted *sandboxGame
	if len(s.games) >= maxSandboxGames {
		for _, old := range s.games {
			if evicted == nil || old.seq < evicted.seq {
				evicted = old
			}
		}
		delete(s.games, evicted.id)
	}
	s.gameSeq++
	sg := &sandboxGame{
		id: fmt.Sprintf("game-%d", s.gameSeq), seq: s.gameSeq, userID: user.ID, saveID: saveID,
		game: g, subs: map[chan []byte]bool{}, recorded: map[string]bool{}, record: s.recordGoals,
	}
	s.games[sg.id] = sg
	s.mu.Unlock()
	if evicted != nil {
		evicted.end()
	}

	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.recordGoals()
	writeJSON(w, http.StatusCreated, sg.state())
}

// recordGoals completes the learning-path exercises tied to goals a game has
// reached. A signed-in player's progress lives in the store (userID); an
// anonymous game records nothing; without a store it is the local progress
// file. A broken path is an error, so the game offers the goals again later;
// the game itself never fails.
func (s *Server) recordGoals(userID, game string, goals []string) error {
	if s.store != nil {
		if userID == "" {
			return nil
		}
		path, err := s.loadPath()
		if err != nil {
			return err
		}
		now := time.Now()
		var probe learning.Progress
		var ids []string
		for _, gl := range goals {
			ids = append(ids, probe.RecordGoal(path, gl, "", now)...)
		}
		if len(ids) == 0 {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = s.store.RecordProgress(ctx, userID, ids, "sandbox "+game, now)
		return err
	}
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	path, progress, err := s.loadLearning()
	if err != nil {
		return err
	}
	var done []string
	for _, gl := range goals {
		done = append(done, progress.RecordGoal(path, gl, "sandbox "+game, time.Now())...)
	}
	if len(done) == 0 {
		return nil
	}
	return progress.Save(s.cfg.ProgressFile)
}

func (s *Server) handleSandboxGet(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	writeJSON(w, http.StatusOK, sg.state())
}

func (s *Server) handleSandboxDelete(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	s.mu.Lock()
	delete(s.games, sg.id)
	s.mu.Unlock()
	sg.end()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSandboxCommand(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	var c sandbox.Command
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid command: %v", err)
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	id, err := sg.game.Apply(c)
	if errors.Is(err, sandbox.ErrInvalid) {
		writeError(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	sg.publish()
	writeJSON(w, http.StatusOK, struct {
		Node  string       `json:"node,omitempty"`
		State SandboxState `json:"state"`
	}{id, sg.state()})
}

func (s *Server) handleSandboxSpeed(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	var req struct {
		Speed int `json:"speed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !sandboxSpeeds[req.Speed] {
		writeError(w, http.StatusBadRequest, "speed must be one of 0, 1, 2, 4, 8")
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.setSpeed(req.Speed)
	sg.publish()
	writeJSON(w, http.StatusOK, sg.state())
}

// handleSandboxStep advances a game by a number of ticks at once.
func (s *Server) handleSandboxStep(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	req := struct {
		Ticks int `json:"ticks"`
	}{Ticks: 1}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: %v", err)
			return
		}
	}
	if req.Ticks < 1 || req.Ticks > 2880 {
		writeError(w, http.StatusBadRequest, "ticks must be between 1 and 2880 (ten simulated days)")
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	for i := 0; i < req.Ticks && sg.game.Status == sandbox.StatusRunning; i++ {
		sg.game.Step()
	}
	sg.publish()
	writeJSON(w, http.StatusOK, sg.state())
}

// handleSandboxSave persists the replayable record. A signed-in player's save
// is owned in the store; without a store it goes to .forgelab/sandbox/. An
// anonymous player in store mode has nowhere to keep it (ADR-0031).
func (s *Server) handleSandboxSave(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	sg.mu.Lock()
	save := sg.game.Save()
	saveID := sg.saveID
	sg.mu.Unlock()

	if s.store != nil {
		user, ok := s.currentUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "sign in to save a sandbox")
			return
		}
		raw, _ := json.Marshal(save)
		var err error
		if saveID == "" {
			var sum store.SandboxSummary
			if sum, err = s.store.CreateSandbox(r.Context(), user.ID, body.Name, save.Ruleset, save.Seed, save.Tick, string(raw)); err == nil {
				saveID = sum.ID
				sg.mu.Lock()
				sg.saveID = sum.ID
				sg.mu.Unlock()
			}
		} else {
			err = s.store.UpdateSandbox(r.Context(), user.ID, saveID, body.Name, save.Tick, string(raw))
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			ID   string       `json:"id,omitempty"`
			Name string       `json:"name,omitempty"`
			Save sandbox.Save `json:"save"`
		}{saveID, body.Name, save})
		return
	}

	dir := filepath.Join(s.cfg.RepoRoot, ".forgelab", "sandbox")
	b, _ := json.MarshalIndent(save, "", "  ")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	path := filepath.Join(dir, sg.id+".json")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Path string       `json:"path"`
		Save sandbox.Save `json:"save"`
	}{path, save})
}

// savedSandboxJSON is a saved sandbox as sent to the dashboard.
type savedSandboxJSON struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Ruleset   string       `json:"ruleset"`
	Seed      int64        `json:"seed"`
	Tick      int          `json:"tick"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Save      sandbox.Save `json:"save"`
}

func savedSummaryJSON(sum store.SandboxSummary) savedSandboxJSON {
	return savedSandboxJSON{ID: sum.ID, Name: sum.Name, Ruleset: sum.Ruleset, Seed: sum.Seed, Tick: sum.Tick, UpdatedAt: sum.UpdatedAt}
}

// requireUser writes 401 and returns false unless the request is signed in.
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	user, ok := s.currentUser(r)
	if s.store == nil || !ok {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return store.User{}, false
	}
	return user, true
}

func (s *Server) handleSandboxSavesList(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	list, err := s.store.ListSandboxes(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	out := make([]savedSandboxJSON, 0, len(list))
	for _, sum := range list {
		out = append(out, savedSummaryJSON(sum))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSandboxSaveGet(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	saved, err := s.store.GetSandbox(r.Context(), user.ID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "unknown save %q", r.PathValue("id"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	out := savedSummaryJSON(saved.SandboxSummary)
	_ = json.Unmarshal([]byte(saved.Save), &out.Save)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSandboxSaveDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteSandbox(r.Context(), user.ID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "unknown save %q", r.PathValue("id"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSandboxStream sends the game state as Server-Sent Events: once on
// connect, then after every tick and command.
func (s *Server) handleSandboxStream(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ch := make(chan []byte, 1)
	sg.mu.Lock()
	sg.subs[ch] = true
	first, _ := json.Marshal(sg.state())
	sg.mu.Unlock()
	defer func() {
		sg.mu.Lock()
		if sg.subs[ch] {
			delete(sg.subs, ch)
		}
		sg.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: %s\n\n", first)
	flusher.Flush()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case b, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
