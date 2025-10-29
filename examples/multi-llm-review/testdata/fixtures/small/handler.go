package main

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Potential nil pointer dereference
	var resp *Response
	
	if r.Method == "POST" {
		resp = &Response{
			Status:  "success",
			Message: "Request processed",
		}
	}
	
	// resp could be nil if method is not POST
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func StartServer() {
	http.HandleFunc("/api", HandleRequest)
	http.ListenAndServe(":8080", nil)
}
