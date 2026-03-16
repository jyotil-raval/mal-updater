package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/token"
)

// RefreshToken exchanges a refresh token for a new access token.
// Returns a new Token on success, or an error if the refresh token
// has expired — caller should fall back to full re-authentication.
func RefreshToken(clientID, refreshToken string) (token.Token, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("grant_type", config.GrantTypeRefresh)
	form.Set("refresh_token", refreshToken)

	resp, err := http.PostForm(config.MALTokenURL, form)
	if err != nil {
		return token.Token{}, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return token.Token{}, fmt.Errorf("refresh failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return token.Token{}, fmt.Errorf("decoding refresh response: %w", err)
	}

	return token.Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}, nil
}
