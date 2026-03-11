package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"github.com/jyotil-raval/mal-updater/internal/auth"
	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/diff"
	"github.com/jyotil-raval/mal-updater/internal/mal"
	"github.com/jyotil-raval/mal-updater/internal/store"
	"github.com/jyotil-raval/mal-updater/internal/updater"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	clientID := os.Getenv("MAL_CLIENT_ID")
	redirectURI := os.Getenv("MAL_REDIRECT_URI")

	if clientID == "" {
		log.Fatal("MAL_CLIENT_ID is not set in .env")
	}
	if redirectURI == "" {
		log.Fatal("MAL_REDIRECT_URI is not set in .env")
	}

	fmt.Println("Environment loaded successfully.")

	// Phase 4 — Auth with Phase 8 refresh layer
	token, err := loadOrRefreshToken(clientID, redirectURI)
	if err != nil {
		log.Fatal(err)
	}

	// Phase 5 — Fetch MAL list
	fmt.Println("\nFetching anime list from MAL...")
	malEntries, err := mal.GetAnimeList(token.AccessToken)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fetched %d entries from MAL\n", len(malEntries))

	// Phase 6 — Load local watchlist + diff
	fmt.Println("\nLoading local watchlist...")
	watchlist, err := diff.LoadWatchlist("watchlist.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Loaded %d entries from watchlist\n", len(watchlist))

	fmt.Println("\nComparing watchlist against MAL...")
	updates, err := diff.Compare(watchlist, malEntries)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d updates needed\n", len(updates))

	if len(updates) == 0 {
		fmt.Println("MAL is already in sync. Nothing to do.")
		return
	}

	// Phase 7 — Apply updates
	fmt.Printf("\nApplying %d updates (concurrency: %d)...\n",
		len(updates), config.MALUpdateConcurrency)

	errs := updater.ApplyUpdates(updates, token.AccessToken)

	fmt.Printf("\nDone. %d succeeded, %d failed.\n",
		len(updates)-len(errs), len(errs))

	if len(errs) > 0 {
		fmt.Println("\nFailed updates:")
		for _, e := range errs {
			log.Printf("  %v", e)
		}
		os.Exit(1)
	}
}

// loadOrRefreshToken handles the full token lifecycle:
//  1. Load existing token from disk
//  2. If valid → use directly
//  3. If expired → attempt silent refresh
//  4. If refresh fails → full re-authentication via browser
func loadOrRefreshToken(clientID, redirectURI string) (store.Token, error) {
	token, err := store.Load()

	if err == nil && !token.IsExpired() {
		fmt.Println("Existing token loaded. Skipping authentication.")
		return token, nil
	}

	// Token expired — attempt silent refresh
	if err == nil && token.IsExpired() {
		fmt.Println("Token expired. Attempting silent refresh...")
		refreshed, refreshErr := auth.RefreshToken(clientID, token.RefreshToken)
		if refreshErr == nil {
			if saveErr := store.Save(refreshed); saveErr != nil {
				return store.Token{}, fmt.Errorf("saving refreshed token: %w", saveErr)
			}
			fmt.Println("Token refreshed successfully.")
			return refreshed, nil
		}
		fmt.Printf("Refresh failed (%v). Falling back to full authentication.\n", refreshErr)
	}

	// No valid token or refresh failed — full auth flow
	fmt.Println("No valid token found. Starting authentication flow...")
	return fullAuthFlow(clientID, redirectURI)
}

// fullAuthFlow runs the complete browser-based OAuth2 PKCE flow
// and returns a saved token on success
func fullAuthFlow(clientID, redirectURI string) (store.Token, error) {
	pkce, err := auth.GeneratePKCE()
	if err != nil {
		return store.Token{}, fmt.Errorf("generating PKCE: %w", err)
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
		return store.Token{}, fmt.Errorf("waiting for callback: %w", err)
	}
	fmt.Println("Authorization code received.")

	token, err := auth.ExchangeCode(clientID, redirectURI, code, pkce.Verifier)
	if err != nil {
		return store.Token{}, fmt.Errorf("exchanging code: %w", err)
	}

	if err := store.Save(token); err != nil {
		return store.Token{}, fmt.Errorf("saving token: %w", err)
	}

	fmt.Println("Token saved successfully.")
	return token, nil
}
