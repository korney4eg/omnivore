package resolver

// This file will not be regenerated automatically.
// Add shared resolver dependencies here; they are populated in Phase 4+.

import (
	"github.com/omnivore-app/omnivore/internal/api/services"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
)

// Resolver is the root dependency-injection struct for all GraphQL resolvers.
type Resolver struct {
	Config   *config.Config
	DB       *db.DB
	Redis    *redisutil.RedisDataSource
	Services *services.Container
}
