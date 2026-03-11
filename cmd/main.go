package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jyotil-raval/mal-updater/internal/auth"
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

	pkce, err := auth.GeneratePKCE()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("PKCE generated successfully.")

	authURL := fmt.Sprintf(
		"https://myanimelist.net/v1/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=S256",
		clientID, redirectURI, pkce.Challenge,
	)

	fmt.Println("\nOpening browser for MAL authentication...")
	if err := auth.OpenBrowser(authURL); err != nil {
		// If auto-open fails, fall back to manual
		fmt.Println("Could not open browser automatically. Open this URL manually:")
		fmt.Println(authURL)
	}

	fmt.Println("Waiting for callback...")

	code, err := auth.WaitForCode("8080")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nAuthorization code received successfully.")
	fmt.Printf("Code : %s\n", code)
}
