package pkg1

import (
	"fmt"
	"time"
)

type Model3 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel3(id string) *Model3 {
	// Missing nil check and validation
	return &Model3{ID: id, Data: make(map[string]interface{})}
}

func (m *Model3) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
