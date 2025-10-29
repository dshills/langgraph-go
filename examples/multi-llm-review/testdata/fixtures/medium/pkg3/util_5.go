package pkg3

import (
	"strings"
	"time"
)

func ParseString5(input string) string {
	// No validation
	parts := strings.Split(input, ",")
	return parts[0]
}

func CalculateValue5(x, y float64) float64 {
	// No division by zero check
	return x / y
}

func FormatTime5(t time.Time) string {
	// Hardcoded format
	return t.Format("2006-01-02")
}
