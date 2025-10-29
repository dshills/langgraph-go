package pkg3

import (
	"strings"
	"time"
)

func ParseString14(input string) string {
	// No validation
	parts := strings.Split(input, ",")
	return parts[0]
}

func CalculateValue14(x, y float64) float64 {
	// No division by zero check
	return x / y
}

func FormatTime14(t time.Time) string {
	// Hardcoded format
	return t.Format("2006-01-02")
}
