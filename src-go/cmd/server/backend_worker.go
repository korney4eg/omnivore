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

	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/queue"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"github.com/omnivore-app/omnivore/internal/storage"
	"github.com/spf13/cobra"
)

var backendWorkerCmd = &cobra.Command{
	Use:   "backend-worker",
	Short: "Start the backend queue worker",
	Long: `Starts the backend queue worker, which processes jobs from the
omnivore-backend-queue:
  - save-page: finalize articles after content fetching
  - update-labels: sync label denormalization
  - trigger-rule: execute user automation rules
  - call-webhook: dispatch registered webhooks
  - refresh-feed: fetch RSS feed content
  - bulk-action: batch operations on library items
  - send-email: send transactional emails
  - prune-trash: clean up deleted items
  - expire-folders: apply folder expiration policies

Configuration is provided via environment variables. See .env.example.`,
	RunE: runBackendWorker,
}

func init() {
	Cmd.AddCommand(backendWorkerCmd)
}

func runBackendWorker(_ *cobra.Command, _ []string) error {
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

	// Connect to blob storage (optional)
	var store *storage.Client
	if cfg.BlobStorageURL != "" {
		var storeErr error
		store, storeErr = storage.New(context.Background(), cfg.BlobStorageURL)
		if storeErr != nil {
			log.Fatalf("failed to open blob storage: %v", storeErr)
		}
		defer store.Close()
	}

	// Start worker
	workerCtx, workerCancel := context.WithCancel(context.Background())
	worker := queue.NewBackendWorker(workerCtx, cfg, database, redisDS, store)
	worker.Start()

	// Start a minimal HTTP server for health checks
	port := cfg.Port
	if port == 0 {
		port = 8081
	}
	addr := ":" + strconv.Itoa(port)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /_ah/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		slog.Info("backend-worker health endpoint listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down backend-worker", "signal", sig.String())

	if err := httpServer.Shutdown(context.Background()); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	workerCancel()
	worker.Wait()
	slog.Info("backend-worker stopped")

	return nil
}
