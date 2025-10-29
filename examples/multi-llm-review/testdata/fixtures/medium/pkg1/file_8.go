package pkg1

import (
	"fmt"
	"time"
)

type Model8 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel8(id string) *Model8 {
	// Missing nil check and validation
	return &Model8{ID: id, Data: make(map[string]interface{})}
}

func (m *Model8) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
