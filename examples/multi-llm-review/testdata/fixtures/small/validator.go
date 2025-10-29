package main

import (
	"regexp"
	"strings"
)

type Validator struct {
	emailRegex *regexp.Regexp
}

func NewValidator() *Validator {
	// Incomplete regex pattern
	regex, _ := regexp.Compile(`^[a-z]+@[a-z]+\.[a-z]+$`)
	return &Validator{emailRegex: regex}
}

// Validator with incomplete logic - intentional issue
func (v *Validator) ValidateEmail(email string) bool {
	// Missing nil check for emailRegex
	// Overly simplistic regex
	return v.emailRegex.MatchString(email)
}

func (v *Validator) ValidatePassword(password string) bool {
	// Weak password validation
	return len(password) > 6
}

func (v *Validator) ValidateUsername(username string) bool {
	// Missing special character check
	return len(strings.TrimSpace(username)) > 0
}
