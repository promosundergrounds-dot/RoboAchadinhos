package storage

import (
	"context"
	"database/sql"
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
	image_url TEXT, -- Adicione esta coluna
	created_at DATETIME
);`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return err
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
	// - Garanta que as colunas aqui batam com o CREATE TABLE
	const statement = `INSERT OR IGNORE INTO offers (id, title, price, url, image_url, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, statement,
		offer.ID,
		offer.Title,
		offer.Price,
		offer.Permalink, // Mapeado para a coluna 'url'
		offer.ImageURL,
		time.Now().UTC(),
	)
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}
