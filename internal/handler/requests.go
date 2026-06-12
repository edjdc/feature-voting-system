package handler

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type RequestsHandler struct {
	requestSvc *service.RequestService
	rankingSvc *service.RankingService
	log        *slog.Logger
}

func NewRequestsHandler(requestSvc *service.RequestService, rankingSvc *service.RankingService, log *slog.Logger) *RequestsHandler {
	return &RequestsHandler{requestSvc: requestSvc, rankingSvc: rankingSvc, log: log}
}

// Submit handles POST /requests.
func (h *RequestsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	fr, err := h.requestSvc.Submit(r.Context(), userID, req.Title, req.Description)
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("submit feature request", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "submission failed")
		return
	}

	writeJSON(w, http.StatusCreated, fr)
}

// List handles GET /requests.
func (h *RequestsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "new"
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	var cursorStr *string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursorStr = &c
	}

	params := service.ListParams{
		Sort:     sort,
		Cursor:   cursorStr,
		Limit:    limit,
		ViewerID: userID,
	}

	page, err := h.rankingSvc.List(r.Context(), params)
	if err != nil {
		h.log.Error("list requests", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "listing failed")
		return
	}

	// ETag for conditional polling
	etag := computeETag(page)
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")

	if match := r.Header.Get("If-None-Match"); match != "" && match == `"`+etag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	writeJSON(w, http.StatusOK, page)
}

// GetByID handles GET /requests/{id}.
func (h *RequestsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")

	fr, err := h.requestSvc.GetByID(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "feature request not found")
			return
		}
		h.log.Error("get feature request", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "retrieval failed")
		return
	}

	writeJSON(w, http.StatusOK, fr)
}

func computeETag(page *service.RequestPage) string {
	h := sha256.New()
	data, _ := json.Marshal(page)
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
