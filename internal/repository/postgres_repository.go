package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
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
// Returns existing shortID and true if originalURL already exists (conflict on original_url unique index)
func (r *PostgresURLRepository) Save(ctx context.Context, shortID, originalURL string) (existingShortID string, conflict bool) {
	// Try to insert
	query := `
		INSERT INTO url_shortener (short_url, original_url)
		VALUES ($1, $2)
		ON CONFLICT (short_url) 
		DO UPDATE SET original_url = EXCLUDED.original_url
	`
	_, err := r.db.ExecContext(ctx, query, shortID, originalURL)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pgerrcode.UniqueViolation {
			if pqErr.Constraint == "idx_original_url" {
				var existingShort string
				getQuery := `SELECT short_url FROM url_shortener WHERE original_url = $1`
				if err := r.db.QueryRowContext(ctx, getQuery, originalURL).Scan(&existingShort); err == nil {
					return existingShort, true
				}
			}
		}
		log.Printf("failed to save URL: %v", err)
		return "", false
	}

	return "", false
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
		log.Printf("failed to get URL: %v", err)
		return "", false
	}

	return originalURL, true
}

// SaveBatch stores multiple URL mappings in a single transaction
func (r *PostgresURLRepository) SaveBatch(ctx context.Context, items []service.BatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO url_shortener (short_url, original_url)
		VALUES ($1, $2)
		ON CONFLICT (short_url) 
		DO UPDATE SET original_url = EXCLUDED.original_url
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx, item.ShortID, item.OriginalURL)
		if err != nil {
			return fmt.Errorf("failed to save URL in batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
