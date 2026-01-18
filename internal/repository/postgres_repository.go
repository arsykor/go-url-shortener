package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// PostgresURLRepository implements service.URLRepository using PostgreSQL
type PostgresURLRepository struct {
	db *sql.DB
}

// NewPostgresURLRepository creates a new PostgreSQL repository
func NewPostgresURLRepository(db *sql.DB) *PostgresURLRepository {
	return &PostgresURLRepository{
		db: db,
	}
}

// Save stores a URL mapping
func (r *PostgresURLRepository) Save(ctx context.Context, shortID, originalURL string) {
	query := `
		INSERT INTO url_shortener (short_url, original_url)
		VALUES ($1, $2)
		ON CONFLICT (short_url) 
		DO UPDATE SET original_url = EXCLUDED.original_url
	`
	_, err := r.db.ExecContext(ctx, query, shortID, originalURL)
	if err != nil {
		_ = fmt.Errorf("failed to save URL: %w", err)
	}
}

// Get retrieves the original URL by short ID
func (r *PostgresURLRepository) Get(ctx context.Context, shortID string) (string, bool) {
	var originalURL string
	query := `SELECT original_url FROM url_shortener WHERE short_url = $1`

	err := r.db.QueryRowContext(ctx, query, shortID).Scan(&originalURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false
		}
		_ = fmt.Errorf("failed to get URL: %w", err)
		return "", false
	}

	return originalURL, true
}
