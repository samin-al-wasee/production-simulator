package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/identity"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

// sessionBackend maps a session cookie value to a user, standing in for OAuth
// in tests. It keys on the SHA-256 of the token, as the real service does.
type sessionBackend struct {
	byTokenHash map[string]store.User
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return string(h[:])
}

func (b sessionBackend) UpsertUserByOAuth(context.Context, string, string, string, string, string) (store.User, error) {
	return store.User{}, nil
}
func (b sessionBackend) CreateSession(context.Context, []byte, string, time.Time) error { return nil }
func (b sessionBackend) DeleteSession(context.Context, []byte) error                    { return nil }
func (b sessionBackend) SessionUser(_ context.Context, hash []byte) (store.User, error) {
	if u, ok := b.byTokenHash[string(hash)]; ok {
		return u, nil
	}
	return store.User{}, store.ErrNotFound
}

// fakeStore is an in-memory appStore that enforces ownership like Postgres does.
type fakeStore struct {
	seq      int
	saves    map[string]store.SavedSandbox
	owner    map[string]string
	progress map[string]map[string]store.ProgressEntry
}

func newFakeStore() *fakeStore {
	return &fakeStore{saves: map[string]store.SavedSandbox{}, owner: map[string]string{}, progress: map[string]map[string]store.ProgressEntry{}}
}

func (f *fakeStore) CreateSandbox(_ context.Context, userID, name, ruleset string, seed int64, tick int, saveJSON string) (store.SandboxSummary, error) {
	f.seq++
	id := fmt.Sprintf("save-%d", f.seq)
	sum := store.SandboxSummary{ID: id, Name: name, Ruleset: ruleset, Seed: seed, Tick: tick, UpdatedAt: time.Now()}
	f.saves[id] = store.SavedSandbox{SandboxSummary: sum, Save: saveJSON}
	f.owner[id] = userID
	return sum, nil
}

func (f *fakeStore) UpdateSandbox(_ context.Context, userID, id, name string, tick int, saveJSON string) error {
	sb, ok := f.saves[id]
	if !ok || f.owner[id] != userID {
		return store.ErrNotFound
	}
	sb.Name, sb.Tick, sb.Save = name, tick, saveJSON
	f.saves[id] = sb
	return nil
}

func (f *fakeStore) ListSandboxes(_ context.Context, userID string) ([]store.SandboxSummary, error) {
	out := []store.SandboxSummary{}
	for id, sb := range f.saves {
		if f.owner[id] == userID {
			out = append(out, sb.SandboxSummary)
		}
	}
	return out, nil
}

func (f *fakeStore) GetSandbox(_ context.Context, userID, id string) (store.SavedSandbox, error) {
	sb, ok := f.saves[id]
	if !ok || f.owner[id] != userID {
		return store.SavedSandbox{}, store.ErrNotFound
	}
	return sb, nil
}

func (f *fakeStore) DeleteSandbox(_ context.Context, userID, id string) error {
	if _, ok := f.saves[id]; !ok || f.owner[id] != userID {
		return store.ErrNotFound
	}
	delete(f.saves, id)
	delete(f.owner, id)
	return nil
}

func (f *fakeStore) ProgressForUser(_ context.Context, userID string) ([]store.ProgressEntry, error) {
	out := []store.ProgressEntry{}
	for _, e := range f.progress[userID] {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeStore) RecordProgress(_ context.Context, userID string, ids []string, by string, at time.Time) ([]string, error) {
	if f.progress[userID] == nil {
		f.progress[userID] = map[string]store.ProgressEntry{}
	}
	var added []string
	for _, id := range ids {
		if _, ok := f.progress[userID][id]; ok {
			continue
		}
		f.progress[userID][id] = store.ProgressEntry{ExerciseID: id, CompletedAt: at, CompletedBy: by}
		added = append(added, id)
	}
	return added, nil
}

// newOwnershipServer builds a store-mode server where cookie "A" is user A and
// cookie "B" is user B.
func newOwnershipServer(t *testing.T) *Server {
	t.Helper()
	backend := sessionBackend{byTokenHash: map[string]store.User{
		tokenHash("A"): {ID: "user-a"},
		tokenHash("B"): {ID: "user-b"},
	}}
	s := newTestServer(t)
	s.cfg.Identity = identity.New(identity.Config{
		Store:     backend,
		Providers: []identity.Provider{identity.GitHub("cid", "sec")},
		PublicURL: "http://api.example",
	})
	s.store = newFakeStore()
	return s
}

func authed(method, path, token, body string) *http.Request {
	var r *strings.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	var req *http.Request
	if r == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, r)
	}
	req.AddCookie(&http.Cookie{Name: identity.SessionCookie, Value: token})
	return req
}

func code(h http.Handler, req *http.Request) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func createGame(t *testing.T, s *Server, token string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, authed("POST", "/api/v1/sandbox/games", token, ""))
	if rec.Code != 201 {
		t.Fatalf("create game as %q = %d", token, rec.Code)
	}
	var state map[string]any
	json.Unmarshal(rec.Body.Bytes(), &state)
	return state["id"].(string)
}

func savesCount(t *testing.T, s *Server, token string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, authed("GET", "/api/v1/sandbox/saves", token, ""))
	if rec.Code != 200 {
		t.Fatalf("list saves as %q = %d", token, rec.Code)
	}
	var list []any
	json.Unmarshal(rec.Body.Bytes(), &list)
	return len(list)
}

func learningDone(t *testing.T, s *Server, token string) float64 {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, authed("GET", "/api/v1/learning", token, ""))
	if rec.Code != 200 {
		t.Fatalf("learning as %q = %d", token, rec.Code)
	}
	var status map[string]any
	json.Unmarshal(rec.Body.Bytes(), &status)
	return status["done"].(float64)
}

func TestOwnershipIsolatesGames(t *testing.T) {
	s := newOwnershipServer(t)
	id := createGame(t, s, "A")

	if c := code(s, authed("GET", "/api/v1/sandbox/games/"+id, "B", "")); c != 404 {
		t.Fatalf("B reading A's game = %d, want 404", c)
	}
	if c := code(s, authed("DELETE", "/api/v1/sandbox/games/"+id, "B", "")); c != 404 {
		t.Fatalf("B deleting A's game = %d, want 404", c)
	}
	if c := code(s, authed("GET", "/api/v1/sandbox/games/"+id, "A", "")); c != 200 {
		t.Fatalf("A reading its game = %d", c)
	}
	if c := code(s, authed("GET", "/api/v1/sandbox/games", "B", "")); c != 200 {
		t.Fatalf("B listing = %d", c)
	}
}

func TestOwnershipSavesResumeAndIsolation(t *testing.T) {
	s := newOwnershipServer(t)
	id := createGame(t, s, "A")

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, authed("POST", "/api/v1/sandbox/games/"+id+"/save", "A", ""))
	if rec.Code != 200 {
		t.Fatalf("save = %d body=%s", rec.Code, rec.Body.String())
	}
	var saveBody map[string]any
	json.Unmarshal(rec.Body.Bytes(), &saveBody)
	saveID, _ := saveBody["id"].(string)
	if saveID == "" {
		t.Fatalf("save returned no id: %s", rec.Body.String())
	}

	if n := savesCount(t, s, "A"); n != 1 {
		t.Fatalf("A saves = %d", n)
	}
	if n := savesCount(t, s, "B"); n != 0 {
		t.Fatalf("B saves = %d (another user's save leaked)", n)
	}
	if c := code(s, authed("GET", "/api/v1/sandbox/saves/"+saveID, "B", "")); c != 404 {
		t.Fatalf("B reading A's save = %d", c)
	}
	if c := code(s, authed("DELETE", "/api/v1/sandbox/saves/"+saveID, "B", "")); c != 404 {
		t.Fatalf("B deleting A's save = %d", c)
	}
	if c := code(s, authed("POST", "/api/v1/sandbox/games", "B", `{"saveId":"`+saveID+`"}`)); c != 404 {
		t.Fatalf("B resuming A's save = %d", c)
	}

	// A resumes and re-saves: the same row is updated, not duplicated.
	resume := httptest.NewRecorder()
	s.ServeHTTP(resume, authed("POST", "/api/v1/sandbox/games", "A", `{"saveId":"`+saveID+`"}`))
	if resume.Code != 201 {
		t.Fatalf("A resuming its save = %d body=%s", resume.Code, resume.Body.String())
	}
	var resumed map[string]any
	json.Unmarshal(resume.Body.Bytes(), &resumed)
	resumedID := resumed["id"].(string)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, authed("POST", "/api/v1/sandbox/games/"+resumedID+"/save", "A", ""))
	if rec.Code != 200 {
		t.Fatalf("re-save = %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &saveBody)
	if got, _ := saveBody["id"].(string); got != saveID {
		t.Fatalf("re-save id = %q, want %q", got, saveID)
	}
	if n := savesCount(t, s, "A"); n != 1 {
		t.Fatalf("A saves after re-save = %d", n)
	}
}

func TestPerUserProgress(t *testing.T) {
	s := newOwnershipServer(t)
	if c := code(s, authed("POST", "/api/v1/learning/s1-a/complete", "A", "")); c != 200 {
		t.Fatalf("A completing = %d", c)
	}
	if got := learningDone(t, s, "A"); got != 1 {
		t.Fatalf("A done = %v", got)
	}
	if got := learningDone(t, s, "B"); got != 0 {
		t.Fatalf("B sees another user's progress: done = %v", got)
	}
	if c := code(s, authed("POST", "/api/v1/learning/s1-a/complete", "A", "")); c != 200 {
		t.Fatalf("A completing twice = %d", c)
	}
	if got := learningDone(t, s, "A"); got != 1 {
		t.Fatalf("A done after repeat = %v", got)
	}
}

func TestAnonymousInStoreModePlaysButCannotPersist(t *testing.T) {
	s := newOwnershipServer(t)

	// Anonymous can create and read a game.
	id := createGame(t, s, "")
	if c := code(s, httptest.NewRequest("GET", "/api/v1/sandbox/games/"+id, nil)); c != 200 {
		t.Fatalf("anonymous read = %d", c)
	}
	// But every persistence path is refused.
	if c := code(s, httptest.NewRequest("POST", "/api/v1/sandbox/games/"+id+"/save", nil)); c != 401 {
		t.Fatalf("anonymous save = %d, want 401", c)
	}
	if c := code(s, httptest.NewRequest("GET", "/api/v1/sandbox/saves", nil)); c != 401 {
		t.Fatalf("anonymous saves list = %d, want 401", c)
	}
	if c := code(s, httptest.NewRequest("POST", "/api/v1/learning/s1-a/complete", nil)); c != 401 {
		t.Fatalf("anonymous complete = %d, want 401", c)
	}
	if c := code(s, httptest.NewRequest("POST", "/api/v1/sandbox/games", strings.NewReader(`{"saveId":"save-1"}`))); c != 401 {
		t.Fatalf("anonymous resume = %d, want 401", c)
	}
	// The learning path still renders, with no progress.
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/learning", nil))
	if rec.Code != 200 {
		t.Fatalf("anonymous learning = %d", rec.Code)
	}
	var status map[string]any
	json.Unmarshal(rec.Body.Bytes(), &status)
	if status["done"] != float64(0) {
		t.Fatalf("anonymous done = %v", status["done"])
	}
}
