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
	fmt.Printf("Verifier  : %s\n", pkce.Verifier)
	fmt.Printf("Challenge : %s\n", pkce.Challenge)
}
