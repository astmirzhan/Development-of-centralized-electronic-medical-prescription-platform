package pharmacy

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/qr"
)

// Handler exposes HTTP endpoints for atomic medication dispensing.
type Handler struct {
	service *Service
	logger  *slog.Logger
}

// NewHandler creates a pharmacy HTTP handler.
func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

// RegisterRoutes mounts pharmacy dispensing endpoints under /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(auth.RequireRole(auth.RolePharmacist)).Post("/dispensing", h.Dispense)
}

// Dispense handles POST /api/v1/dispensing.
func (h *Handler) Dispense(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpserver.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authenticated actor context is required")
		return
	}

	var req DispenseRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	res, err := h.service.Dispense(r.Context(), actor, req)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, res)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidDispenseRequest), errors.Is(err, ErrBrandMismatch):
		httpserver.Error(w, http.StatusBadRequest, "INVALID_DISPENSE_REQUEST", err.Error())
	case errors.Is(err, ErrForbiddenPharmacist):
		httpserver.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, qr.ErrTokenNotFound):
		httpserver.Error(w, http.StatusNotFound, "QR_TOKEN_NOT_FOUND", err.Error())
	case errors.Is(err, qr.ErrTokenExpired):
		httpserver.Error(w, http.StatusGone, "QR_TOKEN_EXPIRED", err.Error())
	case errors.Is(err, qr.ErrTokenAlreadyConsumed),
		errors.Is(err, ErrPrescriptionNotActive),
		errors.Is(err, ErrItemAlreadyDispensed),
		errors.Is(err, ErrInsufficientQuantity):
		httpserver.Error(w, http.StatusConflict, "DISPENSE_CONFLICT", err.Error())
	case errors.Is(err, pki.ErrInvalidSignature),
		errors.Is(err, pki.ErrCertificateRevoked),
		errors.Is(err, pki.ErrSignerIINMismatch),
		errors.Is(err, pki.ErrPayloadMismatch):
		httpserver.Error(w, http.StatusUnprocessableEntity, "EDS_VERIFICATION_FAILED", err.Error())
	default:
		h.logger.Error("dispensing failed", "error", err)
		httpserver.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to dispense medication")
	}
}
