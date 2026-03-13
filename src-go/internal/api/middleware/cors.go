package middleware

import (
	"net/http"
	"strings"
)

// CORS returns middleware that adds CORS headers matching the TypeScript API's
// corsConfig.ts behaviour.  clientURL is the allowed origin (CLIENT_URL env var).
func CORS(clientURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowedOrigin(origin, clientURL) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Set-Cookie")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin, clientURL string) bool {
	if clientURL == "" {
		return true // dev: allow all
	}
	// Allow the configured client URL and any subdomain of it.
	return origin == clientURL || strings.HasSuffix(origin, "."+strings.TrimPrefix(clientURL, "https://"))
}
