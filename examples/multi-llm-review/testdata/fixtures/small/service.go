package main

import (
	"fmt"
	"time"
)

type UserService struct {
	users map[string]string
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[string]string),
	}
}

// Missing context parameter - intentional issue
func (s *UserService) GetUser(userID string) (string, error) {
	// This function should accept context for cancellation/timeouts
	time.Sleep(2 * time.Second)
	
	user, exists := s.users[userID]
	if !exists {
		return "", fmt.Errorf("user not found: %s", userID)
	}
	
	return user, nil
}

func (s *UserService) AddUser(userID, name string) {
	s.users[userID] = name
}
