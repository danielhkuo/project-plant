package api

import (
	"encoding/json"
	"net/http"
)

// errorResponse is the structured error format for the API.
type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, field string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: message, Field: field})
}
