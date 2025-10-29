package pkg1

import (
	"fmt"
	"time"
)

type Model10 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel10(id string) *Model10 {
	// Missing nil check and validation
	return &Model10{ID: id, Data: make(map[string]interface{})}
}

func (m *Model10) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
