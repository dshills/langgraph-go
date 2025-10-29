package pkg1

import (
	"fmt"
	"time"
)

type Model5 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel5(id string) *Model5 {
	// Missing nil check and validation
	return &Model5{ID: id, Data: make(map[string]interface{})}
}

func (m *Model5) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
