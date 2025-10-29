#!/bin/bash

# Generate Package 1: Models and Services (20 files)
for i in {1..20}; do
  cat > pkg1/file_${i}.go << 'EOF'
package pkg1

import (
	"fmt"
	"time"
)

type Model${i} struct {
	ID string
	Data map[string]interface{}
	CreatedAt time.Time
}

func NewModel${i}(id string) *Model${i} {
	// Missing nil check and validation
	return &Model${i}{ID: id, Data: make(map[string]interface{})}
}

func (m *Model${i}) Process() error {
	// Missing error handling
	fmt.Println("Processing", m.ID)
	return nil
}
EOF
  sed -i '' "s/\${i}/$i/g" pkg1/file_${i}.go
done

# Generate Package 2: Handlers and Middleware (20 files)
for i in {1..20}; do
  cat > pkg2/handler_${i}.go << 'EOF'
package pkg2

import (
	"encoding/json"
	"net/http"
)

type Handler${i} struct {
	config string
}

func (h *Handler${i}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Missing error handling
	data := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler${i}) Validate(r *http.Request) bool {
	// Incomplete validation
	return r.Method == "POST"
}
EOF
  sed -i '' "s/\${i}/$i/g" pkg2/handler_${i}.go
done

# Generate Package 3: Utilities and Helpers (20 files)
for i in {1..20}; do
  cat > pkg3/util_${i}.go << 'EOF'
package pkg3

import (
	"strings"
	"time"
)

func ParseString${i}(input string) string {
	// No validation
	parts := strings.Split(input, ",")
	return parts[0]
}

func CalculateValue${i}(x, y float64) float64 {
	// No division by zero check
	return x / y
}

func FormatTime${i}(t time.Time) string {
	// Hardcoded format
	return t.Format("2006-01-02")
}
EOF
  sed -i '' "s/\${i}/$i/g" pkg3/util_${i}.go
done

# Generate Package 4: Database and Repository (20 files)
for i in {1..20}; do
  cat > pkg4/repo_${i}.go << 'EOF'
package pkg4

import (
	"database/sql"
	"fmt"
)

type Repository${i} struct {
	db *sql.DB
}

func (r *Repository${i}) Query(id string) interface{} {
	// SQL injection vulnerability
	query := fmt.Sprintf("SELECT * FROM table WHERE id = '%s'", id)
	rows, _ := r.db.Query(query)
	defer rows.Close()
	return nil
}

func (r *Repository${i}) Insert(data map[string]interface{}) {
	// Missing error handling and validation
	fmt.Println("Inserting data", data)
}
EOF
  sed -i '' "s/\${i}/$i/g" pkg4/repo_${i}.go
done

# Generate Package 5: Configuration and Validation (20 files)
for i in {1..20}; do
  cat > pkg5/config_${i}.go << 'EOF'
package pkg5

import "os"

type Config${i} struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig${i}() *Config${i} {
	// Hardcoded values and missing error handling
	return &Config${i}{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config${i}) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
EOF
  sed -i '' "s/\${i}/$i/g" pkg5/config_${i}.go
done

echo "Generated 100 Go files successfully"
