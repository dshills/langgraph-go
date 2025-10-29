package pkg1

import (
	"fmt"
	"time"
)

type Model7 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel7(id string) *Model7 {
	// Missing nil check and validation
	return &Model7{ID: id, Data: make(map[string]interface{})}
}

func (m *Model7) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
