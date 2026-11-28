package cds

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

// Handler exposes HTTP endpoints for Clinical Decision Support checks.
type Handler struct {
	engine Engine
	logger *slog.Logger
}

// NewHandler creates a CDS HTTP handler.
func NewHandler(engine Engine, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes mounts CDS endpoints under /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/cds/check", h.Check)
}

// Check handles POST /api/v1/cds/check.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if actor, ok := auth.ActorFromContext(r.Context()); ok {
		h.logger.Debug("cds check requested by actor", "actor_id", actor.UserID, "role", actor.Role)
	}

	result, err := h.engine.Evaluate(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRequest):
			httpserver.Error(w, http.StatusBadRequest, "CDS_INVALID_REQUEST", err.Error())
		case errors.Is(err, ErrPatientNotFound):
			httpserver.Error(w, http.StatusNotFound, "PATIENT_NOT_FOUND", err.Error())
		case errors.Is(err, ErrDiagnosisNotFound):
			httpserver.Error(w, http.StatusNotFound, "DIAGNOSIS_NOT_FOUND", err.Error())
		case errors.Is(err, ErrMedicineFormNotFound):
			httpserver.Error(w, http.StatusNotFound, "MEDICINE_FORM_NOT_FOUND", err.Error())
		default:
			h.logger.Error("cds evaluation failed", "error", err)
			httpserver.Error(w, http.StatusInternalServerError, "CDS_INTERNAL_ERROR", "failed to evaluate clinical safety checks")
		}
		return
	}

	httpserver.JSON(w, http.StatusOK, result)
}
