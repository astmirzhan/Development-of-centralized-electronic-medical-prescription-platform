package prescriptions

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
)

// Handler exposes HTTP endpoints for prescription canonicalization and EDS signing.
type Handler struct {
	service *Service
	logger  *slog.Logger
}

// NewHandler creates a prescription HTTP handler.
func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

// RegisterRoutes mounts prescription signing endpoints under /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/prescriptions/{id}/canonical", h.Canonical)
	r.Post("/prescriptions/{id}/sign", h.Sign)
}

type signRequest struct {
	CMSSignature string `json:"cms_signature"`
}

// Canonical handles POST /api/v1/prescriptions/{id}/canonical.
func (h *Handler) Canonical(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpserver.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authenticated actor context is required")
		return
	}

	prescriptionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_PRESCRIPTION_ID", "invalid prescription UUID")
		return
	}

	payload, err := h.service.GetCanonical(r.Context(), prescriptionID, actor)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, payload)
}

// Sign handles POST /api/v1/prescriptions/{id}/sign.
func (h *Handler) Sign(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		httpserver.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authenticated actor context is required")
		return
	}

	prescriptionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_PRESCRIPTION_ID", "invalid prescription UUID")
		return
	}

	var req signRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	res, err := h.service.Sign(r.Context(), prescriptionID, actor, req.CMSSignature)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, res)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidSigningRequest):
		httpserver.Error(w, http.StatusBadRequest, "INVALID_SIGNING_REQUEST", err.Error())
	case errors.Is(err, ErrForbiddenActor):
		httpserver.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, ErrPrescriptionNotFound):
		httpserver.Error(w, http.StatusNotFound, "PRESCRIPTION_NOT_FOUND", err.Error())
	case errors.Is(err, ErrInvalidStatusTransition):
		httpserver.Error(w, http.StatusConflict, "INVALID_PRESCRIPTION_STATUS", err.Error())
	case errors.Is(err, pki.ErrInvalidSignature),
		errors.Is(err, pki.ErrCertificateRevoked),
		errors.Is(err, pki.ErrSignerIINMismatch),
		errors.Is(err, pki.ErrPayloadMismatch):
		httpserver.Error(w, http.StatusUnprocessableEntity, "EDS_VERIFICATION_FAILED", err.Error())
	case errors.Is(err, pki.ErrVerifierUnavailable):
		httpserver.Error(w, http.StatusServiceUnavailable, "EDS_VERIFIER_UNAVAILABLE", err.Error())
	default:
		h.logger.Error("prescription signing request failed", "error", err)
		httpserver.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to process prescription signature")
	}
}
