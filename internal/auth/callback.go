package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jyotil-raval/mal-updater/internal/config"
)

// WaitForCode starts a temporary local HTTP server, waits for the OAuth2
// callback from MAL, extracts the authorization code, then shuts down.
func WaitForCode(port string) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":" + port, Handler: mux}

	mux.HandleFunc(config.CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback URL")
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "Authorization successful. You can close this tab.")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("local server error: %w", err)
		}
	}()

	select {
	case code := <-codeChan:
		server.Shutdown(context.Background())
		return code, nil
	case err := <-errChan:
		server.Shutdown(context.Background())
		return "", err
	}
}
