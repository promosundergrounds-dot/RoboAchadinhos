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
	title TEXT NOT NULL,
	price REAL NOT NULL,
	original_price REAL NOT NULL,
	url TEXT NOT NULL,
	posted_at DATETIME NOT NULL
);
`
	_, err := db.ExecContext(ctx, statement)
	return err
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
	const statement = `INSERT INTO offers (id, title, price, original_price, url, posted_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, statement,
		offer.ID,
		offer.Title,
		offer.Price,
		offer.OriginalPrice,
		offer.Permalink,
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
