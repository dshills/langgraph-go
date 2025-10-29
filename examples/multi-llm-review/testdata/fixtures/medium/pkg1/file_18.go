package pkg1

import (
	"fmt"
	"time"
)

type Model18 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel18(id string) *Model18 {
	// Missing nil check and validation
	return &Model18{ID: id, Data: make(map[string]interface{})}
}

func (m *Model18) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
