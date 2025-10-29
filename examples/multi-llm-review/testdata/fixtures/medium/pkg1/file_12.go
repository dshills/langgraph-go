package pkg1

import (
	"fmt"
	"time"
)

type Model12 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel12(id string) *Model12 {
	// Missing nil check and validation
	return &Model12{ID: id, Data: make(map[string]interface{})}
}

func (m *Model12) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
