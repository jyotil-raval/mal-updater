package handlers

import (
	"encoding/json"
	"net/http"
)

type Handlers struct {
	AccessToken string
}

func NewHandlers(accessToken string) *Handlers {
	return &Handlers{AccessToken: accessToken}
}

// writeJSON is a shared helper — sets Content-Type and encodes the response
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError is a shared helper for error responses
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
