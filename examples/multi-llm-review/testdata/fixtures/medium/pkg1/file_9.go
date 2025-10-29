package pkg1

import (
	"fmt"
	"time"
)

type Model9 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel9(id string) *Model9 {
	// Missing nil check and validation
	return &Model9{ID: id, Data: make(map[string]interface{})}
}

func (m *Model9) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
