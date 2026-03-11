package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"github.com/jyotil-raval/mal-updater/internal/auth"
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
		params.Set("code_challenge_method", "plain")

		authURL := "https://myanimelist.net/v1/oauth2/authorize?" + params.Encode()

		fmt.Println("Opening browser for MAL authentication...")
		if err := auth.OpenBrowser(authURL); err != nil {
			fmt.Println("Could not open browser automatically. Open this URL manually:")
			fmt.Println(authURL)
		}

		fmt.Println("Waiting for callback...")
		code, err := auth.WaitForCode("8080")
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

	fmt.Printf("\nAccess Token : %s...\n", token.AccessToken[:20])
	fmt.Printf("Expires At   : %s\n", token.ExpiresAt.Format("2006-01-02 15:04:05"))
}
