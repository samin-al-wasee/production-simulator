// Package identity implements OAuth-only sign-in and database-backed sessions
// for the application layer (ADR-0031). It resolves a provider identity to an
// application user and issues opaque session tokens; it knows nothing about
// simulation, and no simulation package imports it.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

// SessionCookie and StateCookie are the cookies the flow uses.
const (
	SessionCookie = "forgelab_session"
	StateCookie   = "forgelab_oauth"
)

// DefaultSessionTTL is how long a session stays valid (ADR-0031).
const DefaultSessionTTL = 30 * 24 * time.Hour

// stateTTL bounds the OAuth round trip.
const stateTTL = 10 * time.Minute

// ErrUnauthenticated reports that a request carries no valid session.
var ErrUnauthenticated = errors.New("identity: unauthenticated")

// ErrUnknownProvider reports a provider that is not configured.
var ErrUnknownProvider = errors.New("identity: unknown provider")

// Backend is the persistence the identity service needs. *store.Store satisfies it.
type Backend interface {
	UpsertUserByOAuth(ctx context.Context, provider, providerUserID, email, displayName, avatarURL string) (store.User, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error
	SessionUser(ctx context.Context, tokenHash []byte) (store.User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

// Config configures a Service.
type Config struct {
	Store        Backend
	Providers    []Provider
	PublicURL    string        // base for the OAuth redirect_uri, e.g. http://127.0.0.1:8090
	SessionTTL   time.Duration // defaults to DefaultSessionTTL
	CookieSecure bool          // Secure attribute on cookies
}

// Service runs the OAuth flow and issues sessions.
type Service struct {
	store        Backend
	providers    map[string]Provider
	order        []string
	publicURL    string
	ttl          time.Duration
	cookieSecure bool
}

// New builds a Service. Sign-in is offered only for the providers given.
func New(cfg Config) *Service {
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = DefaultSessionTTL
	}
	s := &Service{
		store:        cfg.Store,
		providers:    map[string]Provider{},
		publicURL:    strings.TrimRight(cfg.PublicURL, "/"),
		ttl:          cfg.SessionTTL,
		cookieSecure: cfg.CookieSecure,
	}
	for _, p := range cfg.Providers {
		if p.Name == "" {
			continue
		}
		if _, seen := s.providers[p.Name]; !seen {
			s.order = append(s.order, p.Name)
		}
		s.providers[p.Name] = p
	}
	return s
}

// Providers returns the offered provider names in configuration order.
func (s *Service) Providers() []string { return append([]string(nil), s.order...) }

// BeginLogin starts the Authorization Code + PKCE flow: it stores a single-use
// state and PKCE verifier in a short-lived cookie and returns the provider's
// authorize URL to redirect to.
func (s *Service) BeginLogin(w http.ResponseWriter, name string) (string, error) {
	p, ok := s.providers[name]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	state, err := randomString(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomString(32)
	if err != nil {
		return "", err
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {p.ClientID},
		"redirect_uri":          {s.redirectURI(name)},
		"scope":                 {strings.Join(p.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	for k, v := range p.ExtraAuthParams {
		q.Set(k, v)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    state + "." + verifier,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(stateTTL.Seconds()),
	})
	sep := "?"
	if strings.Contains(p.AuthURL, "?") {
		sep = "&"
	}
	return p.AuthURL + sep + q.Encode(), nil
}

// CompleteLogin verifies the round trip, exchanges the code, resolves the user,
// and issues a session cookie. stateCookieValue is the raw StateCookie value.
func (s *Service) CompleteLogin(ctx context.Context, w http.ResponseWriter, name string, q url.Values, stateCookieValue string) (store.User, error) {
	p, ok := s.providers[name]
	if !ok {
		return store.User{}, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	if e := q.Get("error"); e != "" {
		return store.User{}, fmt.Errorf("provider returned error %q", e)
	}
	state := q.Get("state")
	parts := strings.SplitN(stateCookieValue, ".", 2)
	if state == "" || len(parts) != 2 ||
		subtle.ConstantTimeCompare([]byte(parts[0]), []byte(state)) != 1 {
		return store.User{}, errors.New("state mismatch")
	}
	verifier := parts[1]
	code := q.Get("code")
	if code == "" {
		return store.User{}, errors.New("missing authorization code")
	}
	accessToken, err := p.exchange(ctx, code, s.redirectURI(name), verifier)
	if err != nil {
		return store.User{}, err
	}
	raw, err := p.userInfo(ctx, accessToken)
	if err != nil {
		return store.User{}, err
	}
	pu, err := p.MapUser(raw)
	if err != nil {
		return store.User{}, fmt.Errorf("map %s user: %w", name, err)
	}
	if pu.ID == "" {
		return store.User{}, fmt.Errorf("%s returned no user id", name)
	}
	user, err := s.store.UpsertUserByOAuth(ctx, name, pu.ID, pu.Email, pu.DisplayName, pu.AvatarURL)
	if err != nil {
		return store.User{}, err
	}
	token, hash, err := newToken()
	if err != nil {
		return store.User{}, err
	}
	expires := time.Now().Add(s.ttl)
	if err := s.store.CreateSession(ctx, hash, user.ID, expires); err != nil {
		return store.User{}, err
	}
	s.setSessionCookie(w, token, expires)
	s.clearStateCookie(w)
	return user, nil
}

// CurrentUser returns the user behind a raw session token, or ErrUnauthenticated.
func (s *Service) CurrentUser(ctx context.Context, token string) (store.User, error) {
	if s.store == nil || token == "" {
		return store.User{}, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	u, err := s.store.SessionUser(ctx, hash[:])
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, ErrUnauthenticated
	}
	if err != nil {
		return store.User{}, err
	}
	return u, nil
}

// Logout revokes the session behind a raw token and clears the cookie.
func (s *Service) Logout(ctx context.Context, w http.ResponseWriter, token string) error {
	if s.store != nil && token != "" {
		hash := sha256.Sum256([]byte(token))
		if err := s.store.DeleteSession(ctx, hash[:]); err != nil {
			return err
		}
	}
	s.clearSessionCookie(w)
	return nil
}

func (s *Service) redirectURI(name string) string {
	return s.publicURL + "/api/v1/auth/callback/" + name
}

func (s *Service) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Service) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// newToken returns a raw session token for the cookie and the SHA-256 hash to store.
func newToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(raw))
	return raw, hash[:], nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
