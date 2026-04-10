package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

// Middleware creates an HTTP middleware that authenticates requests via X-API-Key header.
func Middleware(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")

			identity, err := authenticator.Authenticate(apiKey)
			if err != nil {
				status := http.StatusUnauthorized
				if errors.Is(err, ErrMissingAPIKey) {
					writeAuthError(w, status, "missing API key")
				} else if errors.Is(err, ErrInvalidAPIKey) {
					writeAuthError(w, status, "invalid API key")
				} else if errors.Is(err, ErrExpiredAPIKey) {
					writeAuthError(w, status, "API key expired")
				} else {
					writeAuthError(w, status, "authentication failed")
				}
				return
			}

			ctx := ContextWithDeviceID(r.Context(), identity.DeviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: message})
}
