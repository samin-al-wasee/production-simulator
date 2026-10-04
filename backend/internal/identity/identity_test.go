package identity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

type fakeBackend struct {
	created []byte
	user    store.User
}

func (f *fakeBackend) UpsertUserByOAuth(_ context.Context, _, providerUserID, email, displayName, avatarURL string) (store.User, error) {
	f.user = store.User{ID: "u-" + providerUserID, Email: email, DisplayName: displayName, AvatarURL: avatarURL}
	return f.user, nil
}

func (f *fakeBackend) CreateSession(_ context.Context, tokenHash []byte, _ string, _ time.Time) error {
	f.created = append([]byte(nil), tokenHash...)
	return nil
}

func (f *fakeBackend) SessionUser(_ context.Context, tokenHash []byte) (store.User, error) {
	if bytes.Equal(tokenHash, f.created) {
		return f.user, nil
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeBackend) DeleteSession(_ context.Context, _ []byte) error {
	f.created = nil
	return nil
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func TestPKCEChallenge(t *testing.T) {
	sum := sha256.Sum256([]byte("verifier"))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got := pkceChallenge("verifier"); got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}

func TestNewTokenIsRandomAndHashed(t *testing.T) {
	a, ha, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("tokens repeat")
	}
	if len(ha) != sha256.Size {
		t.Fatalf("hash length = %d", len(ha))
	}
	sum := sha256.Sum256([]byte(a))
	if !bytes.Equal(sum[:], ha) {
		t.Fatal("stored hash is not the SHA-256 of the token")
	}
}

func TestBeginLogin(t *testing.T) {
	svc := New(Config{
		Store:     &fakeBackend{},
		Providers: []Provider{GitHub("cid", "sec")},
		PublicURL: "http://api.example",
	})
	rec := httptest.NewRecorder()
	raw, err := svc.BeginLogin(rec, "github")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "https://github.com/login/oauth/authorize?") {
		t.Fatalf("authorize url = %q", raw)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("state") == "" {
		t.Fatalf("authorize params = %v", q)
	}
	if got := q.Get("redirect_uri"); got != "http://api.example/api/v1/auth/callback/github" {
		t.Fatalf("redirect_uri = %q", got)
	}
	state := cookieValue(rec.Result().Cookies(), StateCookie)
	if state == "" || !strings.Contains(state, ".") {
		t.Fatalf("state cookie = %q", state)
	}
	if _, err := svc.BeginLogin(httptest.NewRecorder(), "nope"); err == nil {
		t.Fatal("unknown provider must error")
	}
}

func TestCompleteLogin(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("code") != "good" {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"bad_code"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"tok"}`)
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "bad token", http.StatusUnauthorized)
				return
			}
			io.WriteString(w, `{"id": 42, "login": "ada", "name": "Ada", "email": "ada@example.com", "avatar_url": "http://a"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	p := GitHub("cid", "sec")
	p.AuthURL = provider.URL + "/authorize"
	p.TokenURL = provider.URL + "/token"
	p.UserInfoURL = provider.URL + "/userinfo"

	be := &fakeBackend{}
	svc := New(Config{Store: be, Providers: []Provider{p}, PublicURL: "http://api.example"})

	login := httptest.NewRecorder()
	if _, err := svc.BeginLogin(login, "github"); err != nil {
		t.Fatal(err)
	}
	stateCookie := cookieValue(login.Result().Cookies(), StateCookie)
	state := strings.SplitN(stateCookie, ".", 2)[0]

	rec := httptest.NewRecorder()
	user, err := svc.CompleteLogin(context.Background(), rec, "github",
		url.Values{"code": {"good"}, "state": {state}}, stateCookie)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "u-42" || user.Email != "ada@example.com" || user.DisplayName != "Ada" {
		t.Fatalf("user = %+v", user)
	}
	if len(be.created) != sha256.Size {
		t.Fatal("no session was stored")
	}
	token := cookieValue(rec.Result().Cookies(), SessionCookie)
	if token == "" {
		t.Fatal("no session cookie was set")
	}
	if _, err := svc.CurrentUser(context.Background(), token); err != nil {
		t.Fatalf("issued token does not resolve: %v", err)
	}

	// A mismatched state is rejected.
	if _, err := svc.CompleteLogin(context.Background(), httptest.NewRecorder(), "github",
		url.Values{"code": {"good"}, "state": {"wrong"}}, stateCookie); err == nil {
		t.Fatal("state mismatch must fail")
	}
}

func TestCurrentUserAndLogout(t *testing.T) {
	be := &fakeBackend{}
	svc := New(Config{Store: be, Providers: []Provider{GitHub("cid", "sec")}, PublicURL: "http://api.example"})
	if _, err := svc.CurrentUser(context.Background(), "nope"); err != ErrUnauthenticated {
		t.Fatalf("unknown token error = %v", err)
	}
	token, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := be.CreateSession(context.Background(), hash, "u1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CurrentUser(context.Background(), token); err != nil {
		t.Fatalf("current user: %v", err)
	}
	if err := svc.Logout(context.Background(), httptest.NewRecorder(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CurrentUser(context.Background(), token); err != ErrUnauthenticated {
		t.Fatalf("after logout error = %v", err)
	}
}
