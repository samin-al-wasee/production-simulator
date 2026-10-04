package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/identity"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

type fakeAuthBackend struct {
	created []byte
	user    store.User
}

func (f *fakeAuthBackend) UpsertUserByOAuth(_ context.Context, _, providerUserID, email, displayName, avatarURL string) (store.User, error) {
	f.user = store.User{ID: "u-" + providerUserID, Email: email, DisplayName: displayName, AvatarURL: avatarURL}
	return f.user, nil
}

func (f *fakeAuthBackend) CreateSession(_ context.Context, hash []byte, _ string, _ time.Time) error {
	f.created = append([]byte(nil), hash...)
	return nil
}

func (f *fakeAuthBackend) SessionUser(_ context.Context, hash []byte) (store.User, error) {
	if bytes.Equal(hash, f.created) {
		return f.user, nil
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeAuthBackend) DeleteSession(_ context.Context, _ []byte) error {
	f.created = nil
	return nil
}

func newAuthServer(t *testing.T) *Server {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"tok"}`)
		case "/userinfo":
			io.WriteString(w, `{"id": 42, "login": "ada", "name": "Ada", "email": "ada@example.com", "avatar_url": "http://a"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)

	p := identity.GitHub("cid", "sec")
	p.TokenURL = provider.URL + "/token"
	p.UserInfoURL = provider.URL + "/userinfo"

	s := newTestServer(t)
	s.cfg.Identity = identity.New(identity.Config{
		Store:     &fakeAuthBackend{},
		Providers: []identity.Provider{p},
		PublicURL: "http://api.example",
	})
	return s
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAuthWithoutIdentity(t *testing.T) {
	s := newTestServer(t)

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/auth/providers", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("providers code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec, _ := do(t, s, "GET", "/api/v1/auth/me", ""); rec.Code != 401 {
		t.Fatalf("me code = %d", rec.Code)
	}
	if rec, _ := do(t, s, "GET", "/api/v1/auth/login/github", ""); rec.Code != 503 {
		t.Fatalf("login code = %d", rec.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	s := newAuthServer(t)

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/auth/providers", nil))
	if !strings.Contains(rec.Body.String(), `"github"`) {
		t.Fatalf("providers body = %q", rec.Body.String())
	}

	loginRec := httptest.NewRecorder()
	s.ServeHTTP(loginRec, httptest.NewRequest("GET", "/api/v1/auth/login/github", nil))
	if loginRec.Code != http.StatusFound {
		t.Fatalf("login code = %d", loginRec.Code)
	}
	if loc := loginRec.Header().Get("Location"); !strings.HasPrefix(loc, "https://github.com/login/oauth/authorize") {
		t.Fatalf("login location = %q", loc)
	}
	stateCookie := findCookie(loginRec.Result().Cookies(), identity.StateCookie)
	if stateCookie == nil {
		t.Fatal("login did not set the state cookie")
	}
	state := strings.SplitN(stateCookie.Value, ".", 2)[0]

	cbRec := httptest.NewRecorder()
	cbReq := httptest.NewRequest("GET", "/api/v1/auth/callback/github?code=good&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(&http.Cookie{Name: identity.StateCookie, Value: stateCookie.Value})
	s.ServeHTTP(cbRec, cbReq)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback code = %d body=%s", cbRec.Code, cbRec.Body.String())
	}
	session := findCookie(cbRec.Result().Cookies(), identity.SessionCookie)
	if session == nil || session.Value == "" {
		t.Fatal("callback did not set a session cookie")
	}

	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.AddCookie(session)
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != 200 {
		t.Fatalf("me code = %d body=%s", meRec.Code, meRec.Body.String())
	}
	var me map[string]any
	json.Unmarshal(meRec.Body.Bytes(), &me)
	if me["displayName"] != "Ada" || me["email"] != "ada@example.com" {
		t.Fatalf("me = %v", me)
	}

	loRec := httptest.NewRecorder()
	loReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	loReq.AddCookie(session)
	s.ServeHTTP(loRec, loReq)
	if loRec.Code != http.StatusNoContent {
		t.Fatalf("logout code = %d", loRec.Code)
	}
	afterRec := httptest.NewRecorder()
	afterReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	afterReq.AddCookie(session)
	s.ServeHTTP(afterRec, afterReq)
	if afterRec.Code != 401 {
		t.Fatalf("me after logout = %d", afterRec.Code)
	}
}

func TestAuthCallbackRejectsBadState(t *testing.T) {
	s := newAuthServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/auth/callback/github?code=good&state=forged", nil)
	req.AddCookie(&http.Cookie{Name: identity.StateCookie, Value: "other.verifier"})
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad state code = %d", rec.Code)
	}
}

func TestCORSCredentials(t *testing.T) {
	s := newTestServer(t)
	s.cfg.AllowedOrigin = "http://localhost:3001"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/api/v1/auth/me", nil))
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("CORS must allow credentials when an origin is configured")
	}
}
