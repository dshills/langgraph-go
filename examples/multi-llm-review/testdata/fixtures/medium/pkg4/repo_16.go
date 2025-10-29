package pkg4

import (
	"database/sql"
	"fmt"
)

type Repository16 struct {
	db *sql.DB
}

func (r *Repository16) Query(id string) interface{} {
	// SQL injection vulnerability
	query := fmt.Sprintf("SELECT * FROM table WHERE id = '%s'", id)
	rows, _ := r.db.Query(query)
	defer rows.Close()
	return nil
}

func (r *Repository16) Insert(data map[string]interface{}) {
	// Missing error handling and validation
	fmt.Println("Inserting data", data)
}
