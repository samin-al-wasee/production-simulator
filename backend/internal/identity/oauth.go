package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxResponse caps how much of a provider response we read.
const maxResponse = 1 << 20

// ProviderUser is a provider identity mapped to our fields.
type ProviderUser struct {
	ID          string
	Email       string
	DisplayName string
	AvatarURL   string
}

// Provider describes one OAuth 2.0 provider.
type Provider struct {
	Name            string
	ClientID        string
	ClientSecret    string
	AuthURL         string
	TokenURL        string
	UserInfoURL     string
	Scopes          []string
	ExtraAuthParams map[string]string
	// MapUser turns a raw userinfo response into a ProviderUser.
	MapUser func([]byte) (ProviderUser, error)
}

// exchange swaps an authorization code for an access token using the PKCE verifier.
func (p Provider) exchange(ctx context.Context, code, redirectURI, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {p.ClientID},
		"code_verifier": {verifier},
	}
	if p.ClientSecret != "" {
		form.Set("client_secret", p.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	body, status, err := do(req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("token exchange: %s: %s", http.StatusText(status), strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token exchange: no access token in response")
	}
	return tok.AccessToken, nil
}

// userInfo fetches the provider's userinfo document.
func (p Provider) userInfo(ctx context.Context, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "forgelab")
	body, status, err := do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("userinfo: %s: %s", http.StatusText(status), strings.TrimSpace(string(body)))
	}
	return body, nil
}

func do(req *http.Request) ([]byte, int, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
