package pkg3

import (
	"strings"
	"time"
)

func ParseString19(input string) string {
	// No validation
	parts := strings.Split(input, ",")
	return parts[0]
}

func CalculateValue19(x, y float64) float64 {
	// No division by zero check
	return x / y
}

func FormatTime19(t time.Time) string {
	// Hardcoded format
	return t.Format("2006-01-02")
}
