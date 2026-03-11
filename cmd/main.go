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

	token, err := store.Load()
	if err == nil && !token.IsExpired() {
		fmt.Println("Existing token loaded. Skipping authentication.")
	} else {
		fmt.Println("No valid token found. Starting authentication flow...")

		pkce, err := auth.GeneratePKCE()
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}
		fmt.Println("Authorization code received.")

		token, err = auth.ExchangeCode(clientID, redirectURI, code, pkce.Verifier)
		if err != nil {
			log.Fatal(err)
		}

		if err := store.Save(token); err != nil {
			log.Fatal(err)
		}

		fmt.Println("Token saved successfully.")
	}

	// Phase 5 — Fetch MAL list
	fmt.Println("Fetching anime list from MAL...")
	malEntries, err := mal.GetAnimeList(token.AccessToken)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fetched %d entries from MAL\n", len(malEntries))

	// Phase 6 — Load local watchlist
	fmt.Println("Loading local watchlist...")
	watchlist, err := diff.LoadWatchlist("watchlist.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Loaded %d entries from watchlist\n", len(watchlist))

	// Phase 6 — Diff
	fmt.Println("Comparing watchlist against MAL...")
	updates, err := diff.Compare(watchlist, malEntries)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d updates needed\n", len(updates))

	for _, u := range updates {
		fmt.Printf("  [%d] %s → status: %s, episodes: %d\n",
			u.AnimeID, u.Title, u.Status, u.Episodes)
	}
}
