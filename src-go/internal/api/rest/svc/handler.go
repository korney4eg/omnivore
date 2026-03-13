// Package svc provides internal service-to-service REST handlers.
// These endpoints are called by background workers (content-fetch, rss-handler, etc.)
// and are not exposed to end users.
package svc

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/omnivore-app/omnivore/internal/api/services"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
)

// Handler groups all svc route handlers.
type Handler struct {
	cfg      *config.Config
	db       *db.DB
	services *services.Container
}

// New returns a Handler that registers its routes on the given mux.
func New(cfg *config.Config, database *db.DB, svc *services.Container) *Handler {
	return &Handler{cfg: cfg, db: database, services: svc}
}

// Register wires all svc routes onto the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Content callbacks from content-fetch worker
	mux.HandleFunc("POST /svc/pubsub/content/search", h.contentSave)

	// Library item lifecycle
	mux.HandleFunc("POST /svc/pubsub/links/create", h.linkCreate)
	mux.HandleFunc("POST /svc/pubsub/links/pruneTrash", h.pruneTrash)
	mux.HandleFunc("POST /svc/pubsub/links/expireFolders", h.expireFolders)

	// Webhooks
	mux.HandleFunc("POST /svc/pubsub/webhooks/trigger/{action}", h.webhookTrigger)

	// RSS
	mux.HandleFunc("POST /svc/pubsub/rss-feed/fetchAll", h.rssFetchAll)

	// User maintenance
	mux.HandleFunc("POST /svc/pubsub/user/prune", h.userPrune)
}

// contentSave handles content saved by the content-fetch worker.
// The worker calls this after fetching and parsing a page.
func (h *Handler) contentSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID        string  `json:"userId"`
		LibraryItemID string  `json:"libraryItemId"`
		Title         string  `json:"title"`
		Content       string  `json:"content"`
		Description   *string `json:"description"`
		Author        *string `json:"author"`
		SiteName      *string `json:"siteName"`
		SiteIcon      *string `json:"siteIcon"`
		Thumbnail     *string `json:"thumbnail"`
		Language      *string `json:"language"`
		WordCount     *int    `json:"wordCount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	item, err := h.services.LibraryItems.UpdatePage(r.Context(), req.LibraryItemID, req.UserID, services.UpdatePageInput{
		Title:       &req.Title,
		Description: req.Description,
		Author:      req.Author,
		State:       strPtr("SUCCEEDED"),
		Thumbnail:   req.Thumbnail,
	})
	if err != nil {
		slog.Error("contentSave: update page", "err", err)
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	// Update content in a separate write since UpdatePage doesn't handle content
	if req.Content != "" {
		if err := h.db.Write.WithContext(r.Context()).
			Table("omnivore.library_item").
			Where("id = ?", req.LibraryItemID).
			Updates(map[string]any{
				"readable_content": req.Content,
				"site_name":       req.SiteName,
				"site_icon":       req.SiteIcon,
				"item_language":   req.Language,
				"word_count":      req.WordCount,
			}).Error; err != nil {
			slog.Error("contentSave: update content", "err", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": item.ID})
}

// linkCreate creates a new library item from a link save request.
func (h *Handler) linkCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  string  `json:"userId"`
		URL     string  `json:"url"`
		Title   string  `json:"title"`
		Folder  string  `json:"folder"`
		Content *string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	folder := "inbox"
	if req.Folder != "" {
		folder = req.Folder
	}
	item, err := h.services.LibraryItems.SavePage(r.Context(), req.UserID, services.SavePageInput{
		URL:     req.URL,
		Title:   req.Title,
		Folder:  folder,
		Content: req.Content,
		State:   "PROCESSING",
	})
	if err != nil {
		slog.Error("linkCreate: save page", "err", err)
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": item.ID})
}

// pruneTrash permanently removes soft-deleted items older than 14 days.
func (h *Handler) pruneTrash(w http.ResponseWriter, r *http.Request) {
	result := h.db.Write.WithContext(r.Context()).
		Exec("DELETE FROM omnivore.library_item WHERE state = 'DELETED' AND deleted_at < NOW() - INTERVAL '14 days'")
	if result.Error != nil {
		slog.Error("pruneTrash", "err", result.Error)
		http.Error(w, "prune failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": result.RowsAffected})
}

// expireFolders applies folder policies (auto-archive, auto-delete).
func (h *Handler) expireFolders(w http.ResponseWriter, r *http.Request) {
	// Folder policies will be implemented when FolderPolicy model is complete
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// webhookTrigger dispatches webhooks for a given action.
func (h *Handler) webhookTrigger(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	var req struct {
		UserID string `json:"userId"`
		ItemID string `json:"itemId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slog.Info("webhookTrigger", "action", action, "userId", req.UserID, "itemId", req.ItemID)
	// Webhook dispatch will be implemented in Phase 7 (queue handlers)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// rssFetchAll triggers a refresh of all active RSS subscriptions.
func (h *Handler) rssFetchAll(w http.ResponseWriter, r *http.Request) {
	// RSS fetching will use subscription service + queue in Phase 7
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// userPrune permanently removes soft-deleted users.
func (h *Handler) userPrune(w http.ResponseWriter, r *http.Request) {
	result := h.db.Write.WithContext(r.Context()).
		Exec("DELETE FROM omnivore.user WHERE status = 'DELETED' AND updated_at < NOW() - INTERVAL '30 days'")
	if result.Error != nil {
		slog.Error("userPrune", "err", result.Error)
		http.Error(w, "prune failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"pruned": result.RowsAffected})
}

// helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func strPtr(s string) *string { return &s }
