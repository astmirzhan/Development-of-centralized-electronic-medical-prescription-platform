package qr

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

// Handler exposes HTTP endpoints for single-use QR token generation.
type Handler struct {
	service *Service
	logger  *slog.Logger
}

// NewHandler creates a QR HTTP handler.
func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

// RegisterRoutes mounts QR endpoints under /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(auth.RequireRole(auth.RolePatient)).Post("/prescription-items/{itemId}/qr", h.Issue)
}

// Issue handles POST /api/v1/prescription-items/{itemId}/qr.
func (h *Handler) Issue(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpserver.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authenticated actor context is required")
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_ITEM_ID", "invalid prescription item UUID")
		return
	}

	res, err := h.service.IssueToken(r.Context(), itemID, actor.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotOwnedByPatient):
			httpserver.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrPrescriptionItemNotFound):
			httpserver.Error(w, http.StatusNotFound, "PRESCRIPTION_ITEM_NOT_FOUND", err.Error())
		case errors.Is(err, ErrItemNotEligibleForQR):
			httpserver.Error(w, http.StatusConflict, "ITEM_NOT_ELIGIBLE", err.Error())
		default:
			h.logger.Error("qr token issuance failed", "error", err)
			httpserver.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate QR token")
		}
		return
	}

	httpserver.JSON(w, http.StatusOK, res)
}
