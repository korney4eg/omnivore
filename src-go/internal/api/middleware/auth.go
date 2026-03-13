package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// Claims mirrors the TypeScript Claims interface exactly so that tokens issued
// by the TypeScript API are accepted by the Go API and vice-versa.
type Claims struct {
	UID             string `json:"uid"`
	UserRole        string `json:"userRole,omitempty"`
	Scope           string `json:"scope,omitempty"`
	Email           string `json:"email,omitempty"`
	System          bool   `json:"system,omitempty"`
	DestroyAfterUse bool   `json:"destroyAfterUse,omitempty"`
	jwt.RegisteredClaims
}

type contextKey int

const claimsKey contextKey = iota

// ClaimsFromContext retrieves the authenticated claims from the request context.
// Returns nil when the request is unauthenticated.
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// AuthMiddleware extracts and verifies credentials from every request and stores
// the resulting Claims in the request context.  Unauthenticated requests are
// allowed through; individual handlers must call ClaimsFromContext and reject
// nil as appropriate.
//
// Authentication order (matches TypeScript api/src/utils/auth.ts):
//  1. Cookie named "auth" containing a JWT
//  2. Authorization: Bearer <jwt>
//  3. Authorization: <uuid>  →  API key lookup (Redis cache → database)
func AuthMiddleware(jwtSecret string, cache *redis.Client, apiKeyLookup APIKeyLookupFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := resolveClaims(r, jwtSecret, cache, apiKeyLookup)
			if claims != nil {
				r = r.WithContext(context.WithValue(r.Context(), claimsKey, claims))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyLookupFunc is called on an API key cache miss to load claims from the
// database.  It returns (nil, nil) when the key is not found or expired.
type APIKeyLookupFunc func(ctx context.Context, keyHash string) (*Claims, time.Time, error)

func resolveClaims(r *http.Request, jwtSecret string, cache *redis.Client, lookup APIKeyLookupFunc) *Claims {
	// 1. Cookie
	if cookie, err := r.Cookie("auth"); err == nil && cookie.Value != "" {
		if c := verifyJWT(cookie.Value, jwtSecret); c != nil {
			return c
		}
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}

	// 2. Bearer JWT
	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		return verifyJWT(strings.TrimSpace(after), jwtSecret)
	}

	// 3. Raw UUID → API key
	raw := strings.TrimSpace(authHeader)
	if raw != "" && lookup != nil {
		return resolveAPIKey(r.Context(), raw, cache, lookup)
	}

	return nil
}

func verifyJWT(tokenStr, secret string) *Claims {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil
	}
	c, _ := token.Claims.(*Claims)
	return c
}

func resolveAPIKey(ctx context.Context, rawKey string, cache *redis.Client, lookup APIKeyLookupFunc) *Claims {
	hash := hashAPIKey(rawKey)
	cacheKey := "api-key:" + hash

	// Try Redis cache first
	if cache != nil {
		if val, err := cache.Get(ctx, cacheKey).Result(); err == nil {
			var c Claims
			if json.Unmarshal([]byte(val), &c) == nil {
				return &c
			}
		}
	}

	// Cache miss — hit the database
	claims, expiresAt, err := lookup(ctx, hash)
	if err != nil || claims == nil {
		return nil
	}

	// Cache the result until the key expires (or 1 hour, whichever is sooner)
	if cache != nil {
		ttl := time.Until(expiresAt)
		if ttl > time.Hour {
			ttl = time.Hour
		}
		if ttl > 0 {
			if data, err := json.Marshal(claims); err == nil {
				_ = cache.Set(ctx, cacheKey, data, ttl).Err()
			}
		}
	}

	return claims
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
