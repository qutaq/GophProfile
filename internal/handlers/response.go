package handlers

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, errMsg string, extras map[string]any) {
	body := map[string]any{"error": errMsg}
	for k, v := range extras {
		body[k] = v
	}
	writeJSON(w, status, body)
}
