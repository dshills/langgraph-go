package pkg1

import (
	"fmt"
	"time"
)

type Model11 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel11(id string) *Model11 {
	// Missing nil check and validation
	return &Model11{ID: id, Data: make(map[string]interface{})}
}

func (m *Model11) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
