package pkg1

import (
	"fmt"
	"time"
)

type Model17 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel17(id string) *Model17 {
	// Missing nil check and validation
	return &Model17{ID: id, Data: make(map[string]interface{})}
}

func (m *Model17) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
