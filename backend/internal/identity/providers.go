package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// ProvidersFromEnv builds the providers whose client credentials are present.
// A provider without credentials is not offered (ADR-0031).
func ProvidersFromEnv() []Provider {
	var ps []Provider
	if id, secret := os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET"); id != "" && secret != "" {
		ps = append(ps, GitHub(id, secret))
	}
	if id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		ps = append(ps, Google(id, secret))
	}
	return ps
}

// GitHub returns the GitHub provider.
func GitHub(clientID, secret string) Provider {
	return Provider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: secret,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		MapUser:      mapGitHubUser,
	}
}

// Google returns the Google provider.
func Google(clientID, secret string) Provider {
	return Provider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: secret,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
		MapUser:      mapGoogleUser,
	}
}

func mapGitHubUser(raw []byte) (ProviderUser, error) {
	var v struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ProviderUser{}, err
	}
	if v.ID == 0 {
		return ProviderUser{}, fmt.Errorf("github user has no id")
	}
	// A private email is omitted by GitHub; email is not a login key, so that
	// is fine (the user is still keyed by id).
	name := v.Name
	if name == "" {
		name = v.Login
	}
	return ProviderUser{
		ID:          strconv.FormatInt(v.ID, 10),
		Email:       v.Email,
		DisplayName: name,
		AvatarURL:   v.AvatarURL,
	}, nil
}

func mapGoogleUser(raw []byte) (ProviderUser, error) {
	var v struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ProviderUser{}, err
	}
	if v.Sub == "" {
		return ProviderUser{}, fmt.Errorf("google user has no sub")
	}
	return ProviderUser{
		ID:          v.Sub,
		Email:       v.Email,
		DisplayName: v.Name,
		AvatarURL:   v.Picture,
	}, nil
}
