package pkg2

import (
	"encoding/json"
	"net/http"
)

type Handler18 struct {
	config string
}

func (h *Handler18) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Missing error handling
	data := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler18) Validate(r *http.Request) bool {
	// Incomplete validation
	return r.Method == "POST"
}
