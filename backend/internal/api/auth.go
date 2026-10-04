package api

import (
	"errors"
	"net/http"

	"github.com/samin-al-wasee/production-simulator/backend/internal/identity"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

// registerAuth wires the OAuth sign-in surface (ADR-0031).
func (s *Server) registerAuth() {
	s.mux.HandleFunc("GET /api/v1/auth/providers", s.handleAuthProviders)
	s.mux.HandleFunc("GET /api/v1/auth/login/{provider}", s.handleAuthLogin)
	s.mux.HandleFunc("GET /api/v1/auth/callback/{provider}", s.handleAuthCallback)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.handleAuthMe)
}

type providerInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type userJSON struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

func publicUser(u store.User) userJSON {
	return userJSON{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL}
}

// currentUser reads the session cookie, if any. It is the seam 13.1c uses to
// make resources user-owned; 13.1b only exposes it.
func (s *Server) currentUser(r *http.Request) (store.User, bool) {
	if s.cfg.Identity == nil {
		return store.User{}, false
	}
	c, err := r.Cookie(identity.SessionCookie)
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	u, err := s.cfg.Identity.CurrentUser(r.Context(), c.Value)
	if err != nil {
		return store.User{}, false
	}
	return u, true
}

func (s *Server) handleAuthProviders(w http.ResponseWriter, _ *http.Request) {
	list := []providerInfo{}
	if s.cfg.Identity != nil {
		for _, name := range s.cfg.Identity.Providers() {
			list = append(list, providerInfo{Name: name, URL: "/api/v1/auth/login/" + name})
		}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Identity == nil {
		writeError(w, http.StatusServiceUnavailable, "sign-in is not configured")
		return
	}
	url, err := s.cfg.Identity.BeginLogin(w, r.PathValue("provider"))
	if err != nil {
		if errors.Is(err, identity.ErrUnknownProvider) {
			writeError(w, http.StatusNotFound, "%v", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Identity == nil {
		writeError(w, http.StatusServiceUnavailable, "sign-in is not configured")
		return
	}
	state := ""
	if c, err := r.Cookie(identity.StateCookie); err == nil {
		state = c.Value
	}
	if _, err := s.cfg.Identity.CompleteLogin(r.Context(), w, r.PathValue("provider"), r.URL.Query(), state); err != nil {
		if errors.Is(err, identity.ErrUnknownProvider) {
			writeError(w, http.StatusNotFound, "%v", err)
			return
		}
		writeError(w, http.StatusBadRequest, "sign-in failed: %v", err)
		return
	}
	http.Redirect(w, r, s.afterLogin(), http.StatusFound)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Identity == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	token := ""
	if c, err := r.Cookie(identity.SessionCookie); err == nil {
		token = c.Value
	}
	if err := s.cfg.Identity.Logout(r.Context(), w, token); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, publicUser(u))
}

// afterLogin is where the callback sends the browser: the dashboard when one is
// configured, otherwise the API root.
func (s *Server) afterLogin() string {
	if s.cfg.AllowedOrigin != "" {
		return s.cfg.AllowedOrigin
	}
	return "/"
}
