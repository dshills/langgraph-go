package pkg1

import (
	"fmt"
	"time"
)

type Model6 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel6(id string) *Model6 {
	// Missing nil check and validation
	return &Model6{ID: id, Data: make(map[string]interface{})}
}

func (m *Model6) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
