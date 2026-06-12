package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type VotesHandler struct {
	voteSvc *service.VoteService
	log     *slog.Logger
}

func NewVotesHandler(voteSvc *service.VoteService, log *slog.Logger) *VotesHandler {
	return &VotesHandler{voteSvc: voteSvc, log: log}
}

// Upvote handles PUT /requests/{id}/vote.
func (h *VotesHandler) Upvote(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	requestID := chi.URLParam(r, "id")

	result, err := h.voteSvc.Upvote(r.Context(), userID, requestID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSelfVote):
			writeError(w, http.StatusForbidden, "forbidden_self_vote", "you cannot vote on your own request")
		case errors.Is(err, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "feature request not found")
		default:
			h.log.Error("upvote", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "vote failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RemoveVote handles DELETE /requests/{id}/vote.
func (h *VotesHandler) RemoveVote(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	requestID := chi.URLParam(r, "id")

	result, err := h.voteSvc.RemoveVote(r.Context(), userID, requestID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "feature request not found")
			return
		}
		h.log.Error("remove vote", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "vote removal failed")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
