package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/sandbox"
)

func sandboxServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	srv := httptest.NewServer(NewServer(Config{RepoRoot: root}))
	t.Cleanup(srv.Close)
	return srv, root
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any, want int, out any) {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, srv.URL+path, r)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		var e map[string]string
		json.NewDecoder(res.Body).Decode(&e)
		t.Fatalf("%s %s: status %d, want %d (%v)", method, path, res.StatusCode, want, e)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
}

type commandResult struct {
	Node  string       `json:"node"`
	State SandboxState `json:"state"`
}

func build(t *testing.T, srv *httptest.Server, id string) {
	t.Helper()
	base := "/api/v1/sandbox/games/" + id + "/commands"
	var app, db commandResult
	call(t, srv, "POST", base, sandbox.Command{Type: sandbox.CmdPlace, Kind: sandbox.KindApp}, 200, &app)
	call(t, srv, "POST", base, sandbox.Command{Type: sandbox.CmdPlace, Kind: sandbox.KindDBPrimary}, 200, &db)
	call(t, srv, "POST", base, sandbox.Command{Type: sandbox.CmdConnect, From: sandbox.InternetID, To: app.Node}, 200, nil)
	call(t, srv, "POST", base, sandbox.Command{Type: sandbox.CmdConnect, From: app.Node, To: db.Node}, 200, nil)
}

func TestSandboxGameLifecycle(t *testing.T) {
	srv, root := sandboxServer(t)
	var rules sandbox.Ruleset
	call(t, srv, "GET", "/api/v1/sandbox/ruleset", nil, 200, &rules)
	if len(rules.Kinds) == 0 {
		t.Fatal("ruleset must list kinds for the build palette")
	}

	var st SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", map[string]int64{"seed": 7}, 201, &st)
	if !st.Simulated || st.Seed != 7 || len(st.Nodes) != 1 || st.Speed != 0 {
		t.Fatalf("new game: %+v", st)
	}
	build(t, srv, st.ID)

	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/commands",
		sandbox.Command{Type: sandbox.CmdConnect, From: "app-instance-1", To: sandbox.InternetID}, 422, nil)
	before := st.Revision
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/step", map[string]int{"ticks": 12}, 200, &st)
	if st.Revision <= before {
		t.Fatalf("revision must increase with every change: %d -> %d", before, st.Revision)
	}
	if st.Tick != 12 || st.Meters.SuccessRPS <= 0 || len(st.History) != 12 {
		t.Fatalf("after 12 ticks: tick=%d meters=%+v", st.Tick, st.Meters)
	}

	var saved struct {
		Path string       `json:"path"`
		Save sandbox.Save `json:"save"`
	}
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/save", nil, 200, &saved)
	if !strings.HasPrefix(saved.Path, root) {
		t.Fatalf("save must land under the repo's .forgelab/: %s", saved.Path)
	}
	if _, err := os.Stat(saved.Path); err != nil {
		t.Fatal(err)
	}
	var replayed SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", map[string]any{"save": saved.Save}, 201, &replayed)
	if replayed.ID == st.ID || !reflect.DeepEqual(replayed.Meters, st.Meters) {
		t.Fatalf("replayed game must match:\n%+v\n%+v", replayed.Meters, st.Meters)
	}

	var list []map[string]any
	call(t, srv, "GET", "/api/v1/sandbox/games", nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("games: %v", list)
	}
	call(t, srv, "DELETE", "/api/v1/sandbox/games/"+st.ID, nil, 204, nil)
	call(t, srv, "GET", "/api/v1/sandbox/games/"+st.ID, nil, 404, nil)
}

func TestSandboxSpeedRunsTheClock(t *testing.T) {
	srv, _ := sandboxServer(t)
	var st SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", nil, 201, &st)
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/speed", map[string]int{"speed": 3}, 400, nil)
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/speed", map[string]int{"speed": 8}, 200, nil)
	deadline := time.Now().Add(3 * time.Second)
	for st.Tick < 2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		call(t, srv, "GET", "/api/v1/sandbox/games/"+st.ID, nil, 200, &st)
	}
	if st.Tick < 2 {
		t.Fatal("the clock should advance at speed 8")
	}
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/speed", map[string]int{"speed": 0}, 200, &st)
	paused := st.Tick
	time.Sleep(300 * time.Millisecond)
	call(t, srv, "GET", "/api/v1/sandbox/games/"+st.ID, nil, 200, &st)
	if st.Tick != paused {
		t.Fatalf("paused game advanced from %d to %d", paused, st.Tick)
	}
}

func TestSandboxStreamSendsStates(t *testing.T) {
	srv, _ := sandboxServer(t)
	var st SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", nil, 201, &st)

	res, err := http.Get(srv.URL + "/api/v1/sandbox/games/" + st.ID + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	events := make(chan SandboxState, 4)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var s SandboxState
				json.Unmarshal([]byte(data), &s)
				events <- s
			}
		}
		close(events)
	}()
	next := func() SandboxState {
		select {
		case s := <-events:
			return s
		case <-time.After(2 * time.Second):
			t.Fatal("no event")
		}
		return SandboxState{}
	}
	if first := next(); first.Tick != 0 {
		t.Fatalf("first event is the current state: %+v", first)
	}
	call(t, srv, "POST", "/api/v1/sandbox/games/"+st.ID+"/step", nil, 200, nil)
	if s := next(); s.Tick != 1 {
		t.Fatalf("expected the stepped state, got tick %d", s.Tick)
	}
	call(t, srv, "DELETE", "/api/v1/sandbox/games/"+st.ID, nil, 204, nil)
	select {
	case _, open := <-events:
		if open {
			t.Fatal("deleting a game should end its stream")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not end")
	}
}

func TestSandboxEvictsTheOldestGame(t *testing.T) {
	srv, _ := sandboxServer(t)
	var first, st SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", nil, 201, &first)
	for i := 1; i < maxSandboxGames; i++ {
		call(t, srv, "POST", "/api/v1/sandbox/games", nil, 201, &st)
	}
	call(t, srv, "POST", "/api/v1/sandbox/games", nil, 201, &st)
	call(t, srv, "GET", "/api/v1/sandbox/games/"+first.ID, nil, 404, nil)
	var list []map[string]any
	call(t, srv, "GET", "/api/v1/sandbox/games", nil, 200, &list)
	if len(list) != maxSandboxGames {
		t.Fatalf("want %d games, got %d", maxSandboxGames, len(list))
	}
}

func TestSandboxEventsAndResponses(t *testing.T) {
	srv, _ := sandboxServer(t)
	var rules sandbox.Ruleset
	call(t, srv, "GET", "/api/v1/sandbox/ruleset", nil, 200, &rules)
	if rules.Version != sandbox.Latest().Version || len(rules.Events) == 0 {
		t.Fatalf("the ruleset must carry the event deck: %s, %d cards", rules.Version, len(rules.Events))
	}

	var st SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", map[string]int64{"seed": 7}, 201, &st)
	if st.Ruleset != rules.Version || st.Events == nil {
		t.Fatalf("new games use the latest ruleset and list events: %s %v", st.Ruleset, st.Events)
	}
	build(t, srv, st.ID)
	base := "/api/v1/sandbox/games/" + st.ID
	call(t, srv, "POST", base+"/commands",
		sandbox.Command{Type: sandbox.CmdRespond, Action: sandbox.ActRestart, Node: "app-instance-1"}, 422, nil)
	call(t, srv, "POST", base+"/step", map[string]int{"ticks": 288 * 10}, 200, &st)
	if len(st.Events) == 0 {
		t.Fatal("ten simulated days should draw events")
	}
	for _, e := range st.Events {
		if e.Label == "" || e.Phase == "" || e.End <= e.Start {
			t.Fatalf("incomplete event: %+v", e)
		}
	}

	var saved struct {
		Save sandbox.Save `json:"save"`
	}
	call(t, srv, "POST", base+"/save", nil, 200, &saved)
	var replayed SandboxState
	call(t, srv, "POST", "/api/v1/sandbox/games", map[string]any{"save": saved.Save}, 201, &replayed)
	if !reflect.DeepEqual(replayed.Events, st.Events) {
		t.Fatal("a replayed game must deal the same events")
	}
}
