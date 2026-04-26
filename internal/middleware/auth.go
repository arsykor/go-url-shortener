// Package middleware provides reusable HTTP middleware for the URL shortener.
package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	// UserIDKey is the context key for the user ID
	UserIDKey contextKey = "userID"
	// cookieName is the name of the authentication cookie
	cookieName = "user_token"
	// secretKey is the key used for HMAC signing
	secretKey = "super-secret-key"
)

var (
	ErrNoUserIDInContext = errors.New("user ID not found in context")
	ErrInvalidUserIDType = errors.New("user ID in context has invalid type")
)

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) (string, error) {
	val := ctx.Value(UserIDKey)
	if val == nil {
		return "", ErrNoUserIDInContext
	}
	userID, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("%w: %T", ErrInvalidUserIDType, val)
	}
	if userID == "" {
		return "", ErrNoUserIDInContext
	}
	return userID, nil
}

// signValue creates an HMAC signature for the given value
func signValue(value string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// buildCookieValue creates a signed cookie value: userID|signature
func buildCookieValue(userID string) string {
	signature := signValue(userID)
	return userID + "|" + signature
}

// parseCookieValue parses and verifies a signed cookie value.
// Returns the userID if valid, empty string otherwise.
func parseCookieValue(cookieValue string) string {
	for i := len(cookieValue) - 1; i >= 0; i-- {
		if cookieValue[i] == '|' {
			userID := cookieValue[:i]
			signature := cookieValue[i+1:]
			expectedSignature := signValue(userID)
			if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
				return userID
			}
			return ""
		}
	}
	return ""
}

// WithAuth is a middleware that manages user authentication via cookies.
// It always sets a valid cookie and puts user ID into context.
func WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID string

		cookie, err := r.Cookie(cookieName)
		if err == nil {
			userID = parseCookieValue(cookie.Value)
		}

		if userID == "" {
			userID = uuid.New().String()
			http.SetCookie(w, &http.Cookie{
				Name:  cookieName,
				Value: buildCookieValue(userID),
				Path:  "/",
			})
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
