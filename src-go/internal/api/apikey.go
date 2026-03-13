package api

import (
	"context"
	"time"

	"github.com/omnivore-app/omnivore/internal/api/middleware"
	"github.com/omnivore-app/omnivore/internal/db"
)

// apiKeyRow is the minimal projection of the api_keys table needed for auth.
type apiKeyRow struct {
	UserID    string    `gorm:"column:user_id"`
	KeyHash   string    `gorm:"column:key_hash"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	Scopes    string    `gorm:"column:scopes"`
}

func (apiKeyRow) TableName() string { return "omnivore.api_keys" }

// buildAPIKeyLookup returns an APIKeyLookupFunc that queries the database for a
// hashed API key and returns the corresponding claims.
func buildAPIKeyLookup(database *db.DB) middleware.APIKeyLookupFunc {
	return func(ctx context.Context, keyHash string) (*middleware.Claims, time.Time, error) {
		var row apiKeyRow
		result := database.Read.WithContext(ctx).
			Where("key_hash = ? AND expires_at > NOW()", keyHash).
			First(&row)
		if result.Error != nil {
			return nil, time.Time{}, nil // not found or expired
		}

		claims := &middleware.Claims{
			UID:   row.UserID,
			Scope: row.Scopes,
		}
		return claims, row.ExpiresAt, nil
	}
}
