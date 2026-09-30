package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/sandbox"
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

	mu    sync.Mutex
	game  *sandbox.Game
	speed int
	// rev increases with every published change so clients can drop a
	// snapshot that arrives after a newer one.
	rev  int
	stop chan struct{}
	subs map[chan []byte]bool
}

// SandboxState is the full game view sent to the dashboard.
type SandboxState struct {
	ID        string           `json:"id"`
	Simulated bool             `json:"simulated"`
	Ruleset   string           `json:"ruleset"`
	Seed      int64            `json:"seed"`
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
		ID: sg.id, Simulated: true, Ruleset: g.Rules.Version, Seed: g.Seed,
		Status: g.Status, Speed: sg.speed, Revision: sg.rev, Tick: g.Tick,
		Meters: g.Last.Meters, Nodes: nodes, Edges: append([]sandbox.Edge{}, g.Edges...),
		Flow: g.Last.Flow, History: append([]sandbox.Meters{}, g.History...), Events: events,
	}
}

// publish must be called with sg.mu held, after every change. Slow
// subscribers miss intermediate states; they always receive a later one.
func (sg *sandboxGame) publish() {
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
}

func (s *Server) sandboxGame(w http.ResponseWriter, r *http.Request) *sandboxGame {
	s.mu.Lock()
	sg := s.games[r.PathValue("id")]
	s.mu.Unlock()
	if sg == nil {
		writeError(w, http.StatusNotFound, "unknown game %q", r.PathValue("id"))
	}
	return sg
}

func (s *Server) handleSandboxRuleset(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, sandbox.Latest())
}

func (s *Server) handleSandboxList(w http.ResponseWriter, _ *http.Request) {
	type summary struct {
		ID     string  `json:"id"`
		Status string  `json:"status"`
		Tick   int     `json:"tick"`
		Users  float64 `json:"users"`
		Cash   float64 `json:"cash"`
	}
	s.mu.Lock()
	games := make([]*sandboxGame, 0, len(s.games))
	for _, sg := range s.games {
		games = append(games, sg)
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

// handleSandboxCreate starts a new empty game, or replays a save when the
// body carries one.
func (s *Server) handleSandboxCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seed *int64        `json:"seed"`
		Save *sandbox.Save `json:"save"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: %v", err)
			return
		}
	}
	var g *sandbox.Game
	switch {
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
		g = sandbox.New(sandbox.Latest(), seed)
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
	sg := &sandboxGame{id: fmt.Sprintf("game-%d", s.gameSeq), seq: s.gameSeq, game: g, subs: map[chan []byte]bool{}}
	s.games[sg.id] = sg
	s.mu.Unlock()
	if evicted != nil {
		evicted.end()
	}

	sg.mu.Lock()
	defer sg.mu.Unlock()
	writeJSON(w, http.StatusCreated, sg.state())
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

// handleSandboxSave writes the replayable record to .forgelab/sandbox/ and
// returns it.
func (s *Server) handleSandboxSave(w http.ResponseWriter, r *http.Request) {
	sg := s.sandboxGame(w, r)
	if sg == nil {
		return
	}
	sg.mu.Lock()
	save := sg.game.Save()
	sg.mu.Unlock()
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
