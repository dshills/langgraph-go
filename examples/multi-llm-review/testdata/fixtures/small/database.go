package main

import (
	"database/sql"
	"fmt"
)

type Database struct {
	conn *sql.DB
}

// Database connection without defer Close() - intentional issue
func NewDatabase(dsn string) (*Database, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	
	// Missing: defer conn.Close() or proper cleanup
	// Missing: conn.Ping() to verify connection
	
	return &Database{conn: conn}, nil
}

func (db *Database) QueryUser(userID string) (string, error) {
	var name string
	// SQL injection vulnerability - not using prepared statement properly
	query := fmt.Sprintf("SELECT name FROM users WHERE id = '%s'", userID)
	err := db.conn.QueryRow(query).Scan(&name)
	return name, err
}
