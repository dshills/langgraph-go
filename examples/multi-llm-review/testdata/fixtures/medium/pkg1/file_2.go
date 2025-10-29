package pkg1

import (
	"fmt"
	"time"
)

type Model2 struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel2(id string) *Model2 {
	// Missing nil check and validation
	return &Model2{ID: id, Data: make(map[string]interface{})}
}

func (m *Model2) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
