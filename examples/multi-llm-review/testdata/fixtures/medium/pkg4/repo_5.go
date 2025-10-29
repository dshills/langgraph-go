package pkg4

import (
	"database/sql"
	"fmt"
)

type Repository5 struct {
	db *sql.DB
}

func (r *Repository5) Query(id string) interface{} {
	// SQL injection vulnerability
	query := fmt.Sprintf("SELECT * FROM table WHERE id = '%s'", id)
	rows, _ := r.db.Query(query)
	defer rows.Close()
	return nil
}

func (r *Repository5) Insert(data map[string]interface{}) {
	// Missing error handling and validation
	fmt.Println("Inserting data", data)
}
