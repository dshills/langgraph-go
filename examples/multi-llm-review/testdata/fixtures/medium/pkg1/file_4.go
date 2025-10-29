package pkg1

import (
	"fmt"
	"time"
)

type Model4 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel4(id string) *Model4 {
	// Missing nil check and validation
	return &Model4{ID: id, Data: make(map[string]interface{})}
}

func (m *Model4) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
