package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file into process environment
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
	fmt.Printf("Client ID : %s\n", clientID)
	fmt.Printf("Redirect  : %s\n", redirectURI)
}
