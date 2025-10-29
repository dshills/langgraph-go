package pkg1

import (
	"fmt"
	"time"
)

type Model16 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel16(id string) *Model16 {
	// Missing nil check and validation
	return &Model16{ID: id, Data: make(map[string]interface{})}
}

func (m *Model16) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
