// token/lifecycle.go
package session

import (
	"fmt"
	"net/url"
	"os"

	"github.com/jyotil-raval/mal-updater/auth"
	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/token"
)

// LoadOrRefresh handles the full token lifecycle:
//  1. Load existing token from disk
//  2. If valid → use directly
//  3. If expired → attempt silent refresh
//  4. If refresh fails → full re-authentication via browser
func LoadOrRefresh() (token.Token, error) {
	clientID := os.Getenv("MAL_CLIENT_ID")
	redirectURI := os.Getenv("MAL_REDIRECT_URI")

	tok, err := token.Load()

	if err == nil && !tok.IsExpired() {
		fmt.Println("Existing token loaded. Skipping authentication.")
		return tok, nil
	}

	if err == nil && tok.IsExpired() {
		fmt.Println("Token expired. Attempting silent refresh...")
		refreshed, refreshErr := auth.RefreshToken(clientID, tok.RefreshToken)
		if refreshErr == nil {
			if saveErr := token.Save(refreshed); saveErr != nil {
				return token.Token{}, fmt.Errorf("saving refreshed token: %w", saveErr)
			}
			fmt.Println("Token refreshed successfully.")
			return refreshed, nil
		}
		fmt.Printf("Refresh failed (%v). Falling back to full authentication.\n", refreshErr)
	}

	fmt.Println("No valid token found. Starting authentication flow...")
	return fullAuthFlow(clientID, redirectURI)
}

// fullAuthFlow runs the complete browser-based OAuth2 PKCE flow
// and returns a saved token on success
func fullAuthFlow(clientID, redirectURI string) (token.Token, error) {
	pkce, err := auth.GeneratePKCE()
	if err != nil {
		return token.Token{}, fmt.Errorf("generating PKCE: %w", err)
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", config.PKCEMethod)

	authURL := config.MALAuthURL + "?" + params.Encode()

	fmt.Println("Opening browser for MAL authentication...")
	if err := auth.OpenBrowser(authURL); err != nil {
		fmt.Println("Could not open browser automatically. Open this URL manually:")
		fmt.Println(authURL)
	}

	fmt.Println("Waiting for callback...")
	code, err := auth.WaitForCode(config.CallbackPort)
	if err != nil {
		return token.Token{}, fmt.Errorf("waiting for callback: %w", err)
	}
	fmt.Println("Authorization code received.")

	tok, err := auth.ExchangeCode(clientID, redirectURI, code, pkce.Verifier)
	if err != nil {
		return token.Token{}, fmt.Errorf("exchanging code: %w", err)
	}

	if err := token.Save(tok); err != nil {
		return token.Token{}, fmt.Errorf("saving token: %w", err)
	}

	fmt.Println("Token saved successfully.")
	return tok, nil
}
