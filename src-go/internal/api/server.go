package api

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/omnivore-app/omnivore/internal/api/graphql/generated"
	"github.com/omnivore-app/omnivore/internal/api/graphql/resolver"
	"github.com/omnivore-app/omnivore/internal/api/middleware"
	"github.com/omnivore-app/omnivore/internal/api/rest/auth"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
)

// Server is the root HTTP handler for the Omnivore API.
type Server struct {
	cfg     *config.Config
	db      *db.DB
	redis   *redisutil.RedisDataSource
	handler http.Handler
}

// New wires up all routes and middleware and returns a ready-to-serve Server.
func New(cfg *config.Config, database *db.DB, redis *redisutil.RedisDataSource) *Server {
	s := &Server{cfg: cfg, db: database, redis: redis}
	s.handler = s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health check (no auth required)
	mux.HandleFunc("GET /_ah/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Auth routes
	authH := auth.New(s.cfg, s.db, s.redis)
	mux.HandleFunc("POST /api/login", authH.Login)
	mux.HandleFunc("POST /api/logout", authH.Logout)
	mux.HandleFunc("POST /api/sign-up", authH.SignUp)
	mux.HandleFunc("GET /api/verify-email", authH.VerifyEmail)
	mux.HandleFunc("POST /api/forgot-password", authH.ForgotPassword)
	mux.HandleFunc("POST /api/reset-password", authH.ResetPassword)
	mux.HandleFunc("POST /api/change-password", authH.ChangePassword)

	// Internal service-to-service routes will be wired here in Phase 6.

	// GraphQL endpoint
	gqlHandler := handler.NewDefaultServer(
		generated.NewExecutableSchema(generated.Config{
			Resolvers: &resolver.Resolver{
				Config: s.cfg,
				DB:     s.db,
				Redis:  s.redis,
			},
		}),
	)
	mux.Handle("POST /graphql", gqlHandler)
	mux.Handle("GET /graphql", gqlHandler)

	// GraphQL playground (local development only)
	if s.cfg.APIEnv == "local" || s.cfg.APIEnv == "" {
		mux.Handle("GET /playground", playground.Handler("Omnivore GraphQL", "/graphql"))
	}

	// Apply global middleware (outermost → innermost): Logging → Auth → CORS → mux
	apiKeyLookup := buildAPIKeyLookup(s.db)
	return middleware.Logging(
		middleware.AuthMiddleware(s.cfg.JWTSecret, s.redis.CacheClient, apiKeyLookup)(
			middleware.CORS(s.cfg.ClientURL)(mux),
		),
	)
}
