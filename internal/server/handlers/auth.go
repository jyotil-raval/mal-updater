package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (h *Handlers) IssueToken(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		writeError(w, http.StatusInternalServerError, "JWT_SECRET not configured")
		return
	}

	expiresIn := int64(86400) // 24 hours in seconds

	claims := jwt.MapClaims{
		"iss": "mal-updater",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      signed,
		"expires_in": expiresIn,
	})
}
