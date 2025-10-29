package main

import "time"

// Struct without validation - intentional issue
type User struct {
	ID        string
	Email     string
	Password  string
	CreatedAt time.Time
}

type Account struct {
	UserID  string
	Balance float64
	Active  bool
}

// No validation for required fields
func NewUser(id, email, password string) *User {
	return &User{
		ID:        id,
		Email:     email,
		Password:  password,
		CreatedAt: time.Now(),
	}
}

func (u *User) IsValid() bool {
	// Incomplete validation
	return u.ID != ""
}
