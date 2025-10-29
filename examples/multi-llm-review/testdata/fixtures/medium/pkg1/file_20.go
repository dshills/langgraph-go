package pkg1

import (
	"fmt"
	"time"
)

type Model20 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel20(id string) *Model20 {
	// Missing nil check and validation
	return &Model20{ID: id, Data: make(map[string]interface{})}
}

func (m *Model20) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
