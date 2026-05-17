package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"log/slog"
	"underground/robo-achadinhos/internal/models"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewStorage(path string, logger *slog.Logger) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	if err := createSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db, logger: logger}, nil
}

func createSchema(ctx context.Context, db *sql.DB) error {
	const statement = `
CREATE TABLE IF NOT EXISTS offers (
	id TEXT PRIMARY KEY,
	title TEXT,
	price REAL,
	url TEXT,
	created_at DATETIME
);
`

	if _, err := db.ExecContext(ctx, statement); err != nil {
		return err
	}

	// Ensure `title` column exists on older DBs that might lack it.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(offers)")
	if err != nil {
		return err
	}
	defer rows.Close()

	foundTitle := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "title" {
			foundTitle = true
			break
		}
	}

	if !foundTitle {
		if _, err := db.ExecContext(ctx, "ALTER TABLE offers ADD COLUMN title TEXT"); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) IsNewOffer(ctx context.Context, id string) (bool, error) {
	const query = `SELECT 1 FROM offers WHERE id = ? LIMIT 1`
	var got int
	if err := s.db.QueryRowContext(ctx, query, id).Scan(&got); err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (s *Storage) MarkAsPosted(ctx context.Context, offer models.Offer) error {
	const statement = `INSERT OR IGNORE INTO offers (id, title, price, url, image_url, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, statement,
		offer.ID,
		offer.Title,
		offer.Price,
		offer.Permalink,
		offer.ImageURL,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to save offer %s: %w", offer.ID, err)
	}
	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}
