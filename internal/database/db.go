package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB holds the database connection
type DB struct {
	conn *sql.DB
}

// NewDB initializes the SQLite database and creates the offers table if it doesn't exist
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create offers table
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS offers (
		id TEXT PRIMARY KEY,
		posted_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = conn.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &DB{conn: conn}, nil
}

// IsNew checks if the product ID is new (not posted recently)
// For simplicity, considers "recently" as within the last 24 hours
func (db *DB) IsNew(id string) (bool, error) {
	var postedAt time.Time
	query := "SELECT posted_at FROM offers WHERE id = ?"
	err := db.conn.QueryRow(query, id).Scan(&postedAt)
	if err == sql.ErrNoRows {
		return true, nil // Not found, so new
	}
	if err != nil {
		return false, fmt.Errorf("failed to query database: %w", err)
	}

	// Check if posted more than 24 hours ago
	if time.Since(postedAt) > 24*time.Hour {
		return true, nil // Consider it new if old
	}
	return false, nil // Recently posted
}

// MarkAsPosted inserts or updates the offer as posted
func (db *DB) MarkAsPosted(id string) error {
	query := "INSERT OR REPLACE INTO offers (id, posted_at) VALUES (?, ?)"
	_, err := db.conn.Exec(query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark as posted: %w", err)
	}
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}