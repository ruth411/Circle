package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

func NewServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", healthz)
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service": "circle",
		"status":  "ok",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}
