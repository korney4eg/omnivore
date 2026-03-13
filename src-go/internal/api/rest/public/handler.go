// Package public provides user-facing REST API endpoints outside of GraphQL.
package public

import (
	"encoding/json"
	"net/http"

	"github.com/omnivore-app/omnivore/internal/api/middleware"
	"github.com/omnivore-app/omnivore/internal/api/services"
	"github.com/omnivore-app/omnivore/internal/config"
)

// Handler groups public REST route handlers.
type Handler struct {
	cfg      *config.Config
	services *services.Container
}

// New returns a new public Handler.
func New(cfg *config.Config, svc *services.Container) *Handler {
	return &Handler{cfg: cfg, services: svc}
}

// Register wires all public REST routes onto the mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Article content endpoint - used by mobile and extensions
	mux.HandleFunc("GET /api/article/{id}", h.getArticle)

	// Content download endpoint
	mux.HandleFunc("POST /api/content", h.getContent)

	// Task status (stub)
	mux.HandleFunc("GET /api/tasks/{id}", h.getTaskStatus)
}

// getArticle returns article content as JSON.
func (h *Handler) getArticle(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	item, err := h.services.LibraryItems.GetByID(r.Context(), id, claims.UID)
	if err != nil || item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      item.ID,
		"title":   item.Title,
		"url":     item.OriginalURL,
		"content": item.ReadableContent,
		"slug":    item.Slug,
	})
}

// getContent returns signed URLs for downloading article content.
func (h *Handler) getContent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		LibraryItemIDs []string `json:"libraryItemIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	type contentResult struct {
		LibraryItemID string `json:"libraryItemId"`
		Content       string `json:"content"`
	}
	results := make([]contentResult, 0, len(req.LibraryItemIDs))
	for _, id := range req.LibraryItemIDs {
		item, err := h.services.LibraryItems.GetByID(r.Context(), id, claims.UID)
		if err != nil || item == nil {
			continue
		}
		results = append(results, contentResult{
			LibraryItemID: id,
			Content:       item.ReadableContent,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// getTaskStatus returns the status of a background task.
func (h *Handler) getTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    id,
		"state": "SUCCEEDED",
	})
}
