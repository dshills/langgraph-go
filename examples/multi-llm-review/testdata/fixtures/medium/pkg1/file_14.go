package pkg1

import (
	"fmt"
	"time"
)

type Model14 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel14(id string) *Model14 {
	// Missing nil check and validation
	return &Model14{ID: id, Data: make(map[string]interface{})}
}

func (m *Model14) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
