package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

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
func (r *PostgresURLRepository) Save(ctx context.Context, shortID, originalURL, userID string) (existingShortID string, conflict bool) {
	// Try to insert
	query := `
		INSERT INTO url_shortener (short_url, original_url, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (short_url) 
		DO UPDATE SET original_url = EXCLUDED.original_url
	`
	_, err := r.db.ExecContext(ctx, query, shortID, originalURL, userID)
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

// Get retrieves the original URL by short ID.
// Returns originalURL, isDeleted flag, and whether the record exists.
func (r *PostgresURLRepository) Get(ctx context.Context, shortID string) (string, bool, bool) {
	var originalURL string
	var isDeleted bool
	query := `SELECT original_url, is_deleted FROM url_shortener WHERE short_url = $1`

	err := r.db.QueryRowContext(ctx, query, shortID).Scan(&originalURL, &isDeleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, false
		}
		log.Printf("failed to get URL: %v", err)
		return "", false, false
	}

	return originalURL, isDeleted, true
}

// SaveBatch stores multiple URL mappings in a single transaction
func (r *PostgresURLRepository) SaveBatch(ctx context.Context, items []service.BatchItem, userID string) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO url_shortener (short_url, original_url, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (short_url) 
		DO UPDATE SET original_url = EXCLUDED.original_url
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx, item.ShortID, item.OriginalURL, userID)
		if err != nil {
			return fmt.Errorf("failed to save URL in batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetURLsByUser returns all URLs shortened by a specific user
func (r *PostgresURLRepository) GetURLsByUser(ctx context.Context, userID string) ([]service.UserURL, error) {
	query := `SELECT short_url, original_url FROM url_shortener WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query URLs by user: %w", err)
	}
	defer rows.Close()

	var result []service.UserURL
	for rows.Next() {
		var u service.UserURL
		if err := rows.Scan(&u.ShortURL, &u.OriginalURL); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// DeleteURLs marks URLs as deleted using a batch UPDATE.
// Only URLs belonging to the given userID are affected.
func (r *PostgresURLRepository) DeleteURLs(ctx context.Context, shortIDs []string, userID string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(shortIDs))
	args := make([]interface{}, 0, len(shortIDs)+1)
	args = append(args, userID)

	for i, id := range shortIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}

	query := fmt.Sprintf(
		`UPDATE url_shortener SET is_deleted = TRUE WHERE user_id = $1 AND short_url IN (%s)`,
		strings.Join(placeholders, ", "),
	)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete URLs: %w", err)
	}

	return nil
}
