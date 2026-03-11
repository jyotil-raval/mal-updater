package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const tokenFile = "token.json"

// Token holds the OAuth2 token response from MAL
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// IsExpired returns true if the access token is within 5 minutes of expiry
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-5 * time.Minute))
}

// Save writes the token to disk as JSON with owner-only permissions
func Save(t Token) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}
	if err := os.WriteFile(tokenFile, data, 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

// Load reads and parses the token from disk
func Load() (Token, error) {
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return Token{}, fmt.Errorf("reading token file: %w", err)
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return Token{}, fmt.Errorf("parsing token file: %w", err)
	}
	return t, nil
}
