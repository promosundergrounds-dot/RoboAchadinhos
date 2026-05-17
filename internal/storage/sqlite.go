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
	meli_id TEXT PRIMARY KEY,
	title TEXT,
	price REAL,
	original_price REAL,
	url TEXT,
	image_url TEXT,
	coupon TEXT,
	category TEXT,
	affiliate_url TEXT,
	created_at DATETIME,
	updated_at DATETIME
);`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return err
	}

	// Ensure all required columns exist (backward compatibility)
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(offers)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingCols := make(map[string]bool)
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
		existingCols[name] = true
	}

	// Add missing columns if they don't exist
	columnsToAdd := map[string]string{
		"coupon":         "TEXT",
		"category":       "TEXT",
		"affiliate_url":  "TEXT",
		"updated_at":     "DATETIME",
		"original_price": "REAL",
	}

	for col, colType := range columnsToAdd {
		if !existingCols[col] {
			addColStmt := fmt.Sprintf("ALTER TABLE offers ADD COLUMN %s %s", col, colType)
			if _, err := db.ExecContext(ctx, addColStmt); err != nil {
				// Silently ignore if column already exists or cannot be added
				_ = err
			}
		}
	}

	if !existingCols["meli_id"] {
		if _, err := db.ExecContext(ctx, "ALTER TABLE offers ADD COLUMN meli_id TEXT"); err != nil {
			_ = err
		}
		if existingCols["id"] {
			if _, err := db.ExecContext(ctx, "UPDATE offers SET meli_id = id WHERE meli_id IS NULL"); err != nil {
				return err
			}
		}
	}

	if !existingCols["url"] && existingCols["permalink"] {
		if _, err := db.ExecContext(ctx, "ALTER TABLE offers ADD COLUMN url TEXT"); err != nil {
			_ = err
		}
		if _, err := db.ExecContext(ctx, "UPDATE offers SET url = permalink WHERE url IS NULL"); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) IsNewOffer(ctx context.Context, id string) (bool, error) {
	const query = `SELECT 1 FROM offers WHERE meli_id = ? LIMIT 1`
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
	meliID := offer.MeliID
	if meliID == "" {
		meliID = offer.ID
	}

	now := time.Now().UTC()
	const statement = `INSERT OR REPLACE INTO offers 
		(meli_id, title, price, original_price, url, image_url, coupon, category, affiliate_url, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM offers WHERE meli_id = ?), ?), ?)`

	_, err := s.db.ExecContext(ctx, statement,
		meliID,
		offer.Title,
		offer.Price,
		offer.OriginalPrice,
		offer.Permalink,
		offer.ImageURL,
		offer.Coupon,
		offer.Category,
		offer.AffiliateLink,
		meliID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to save offer %s: %w", meliID, err)
	}
	return nil
}

func (s *Storage) ListOffers(ctx context.Context) ([]models.Offer, error) {
	const query = `SELECT meli_id, title, price, original_price, url, image_url, coupon, category, affiliate_url FROM offers ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	offers := make([]models.Offer, 0)
	for rows.Next() {
		var offer models.Offer
		if err := rows.Scan(&offer.MeliID, &offer.Title, &offer.Price, &offer.OriginalPrice, &offer.Permalink, &offer.ImageURL, &offer.Coupon, &offer.Category, &offer.AffiliateLink); err != nil {
			return nil, err
		}
		offer.ID = offer.MeliID
		offers = append(offers, offer)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return offers, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}
