package pkg3

import (
	"strings"
	"time"
)

func ParseString11(input string) string {
	// No validation
	parts := strings.Split(input, ",")
	return parts[0]
}

func CalculateValue11(x, y float64) float64 {
	// No division by zero check
	return x / y
}

func FormatTime11(t time.Time) string {
	// Hardcoded format
	return t.Format("2006-01-02")
}
