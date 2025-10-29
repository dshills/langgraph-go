package pkg1

import (
	"fmt"
	"time"
)

type Model13 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel13(id string) *Model13 {
	// Missing nil check and validation
	return &Model13{ID: id, Data: make(map[string]interface{})}
}

func (m *Model13) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
