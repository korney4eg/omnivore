package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	api "github.com/omnivore-app/omnivore/internal/api"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"github.com/omnivore-app/omnivore/internal/storage"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the GraphQL/REST API server",
	Long: `Starts the Omnivore API service, which provides:
  - GraphQL API for web and mobile clients
  - REST endpoints for auth and internal service calls
  - BullMQ job dispatcher for backend queue processing

Configuration is provided via environment variables.`,
	RunE: runAPI,
}

func init() {
	Cmd.AddCommand(apiCmd)
}

func runAPI(_ *cobra.Command, _ []string) error {
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	// Connect to Redis
	redisDS, err := redisutil.New(cfg)
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	defer redisDS.Shutdown()

	// Connect to PostgreSQL
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	// Connect to blob storage (optional — empty URL disables storage features)
	var store *storage.Client
	if cfg.BlobStorageURL != "" {
		var storeErr error
		store, storeErr = storage.New(context.Background(), cfg.BlobStorageURL)
		if storeErr != nil {
			log.Fatalf("failed to open blob storage: %v", storeErr)
		}
		defer store.Close()
	}

	// Build the HTTP server
	apiServer := api.New(cfg, database, redisDS, store)

	port := cfg.APIPort
	if port == 0 {
		port = 4000
	}
	addr := ":" + strconv.Itoa(port)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: apiServer,
	}

	go func() {
		slog.Info("api server listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	if err := httpServer.Shutdown(context.Background()); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	return nil
}
