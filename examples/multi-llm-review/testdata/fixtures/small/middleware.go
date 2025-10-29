package main

import (
	"fmt"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		fmt.Printf("Request: %s %s\n", r.Method, r.URL.Path)
		
		next.ServeHTTP(w, r)
		
		// Missing error check from ServeHTTP
		duration := time.Since(start)
		fmt.Printf("Duration: %v\n", duration)
	})
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		
		// Weak authentication check
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
