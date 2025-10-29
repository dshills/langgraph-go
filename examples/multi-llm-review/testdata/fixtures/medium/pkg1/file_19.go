package pkg1

import (
	"fmt"
	"time"
)

type Model19 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel19(id string) *Model19 {
	// Missing nil check and validation
	return &Model19{ID: id, Data: make(map[string]interface{})}
}

func (m *Model19) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
