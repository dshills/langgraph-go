package pkg2

import (
	"encoding/json"
	"net/http"
)

type Handler12 struct {
	config string
}

func (h *Handler12) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Missing error handling
	data := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler12) Validate(r *http.Request) bool {
	// Incomplete validation
	return r.Method == "POST"
}
