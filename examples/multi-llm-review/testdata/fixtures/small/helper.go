package main

import (
	"fmt"
	"time"
)

// Helper function with hardcoded values - intentional issue
func GetAPIEndpoint() string {
	// Hardcoded URL instead of configuration
	return "https://api.example.com/v1"
}

func GetTimeout() time.Duration {
	// Hardcoded timeout value
	return 30 * time.Second
}

func FormatMessage(userID string, message string) string {
	// Hardcoded format string
	return fmt.Sprintf("[USER:%s] %s", userID, message)
}

func GetMaxRetries() int {
	// Magic number without explanation
	return 3
}

func IsProduction() bool {
	// Hardcoded environment check
	return false
}

func GetDatabaseURL() string {
	// Hardcoded credentials - security issue
	return "mysql://root:password123@localhost:3306/mydb"
}
