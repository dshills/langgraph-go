package main

import "strings"

// Inefficient loop - intentional issue
func ProcessItems(items []string) []string {
	var result []string
	
	for _, item := range items {
		// Inefficient string concatenation in loop
		processed := ""
		for i := 0; i < len(item); i++ {
			processed += strings.ToUpper(string(item[i]))
		}
		result = append(result, processed)
	}
	
	return result
}

func FindDuplicates(items []string) []string {
	// O(n²) complexity - inefficient
	var duplicates []string
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i] == items[j] {
				duplicates = append(duplicates, items[i])
			}
		}
	}
	return duplicates
}
