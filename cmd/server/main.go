package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	server "github.com/jyotil-raval/mal-updater/internal/server"
	"github.com/jyotil-raval/mal-updater/internal/server/handlers"
	"github.com/jyotil-raval/mal-updater/internal/session"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	if os.Getenv("MAL_CLIENT_ID") == "" {
		log.Fatal("MAL_CLIENT_ID is not set in .env")
	}
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET is not set in .env")
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	tok, err := session.LoadOrRefresh()
	if err != nil {
		log.Fatalf("authentication failed: %v", err)
	}

	h := handlers.NewHandlers(tok.AccessToken)
	r := server.NewRouter(h)

	fmt.Printf("Server running on :%s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
