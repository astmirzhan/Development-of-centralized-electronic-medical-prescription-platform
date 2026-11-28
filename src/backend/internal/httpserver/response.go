// Package httpserver wires the chi HTTP router, middlewares, and JSON helpers.
package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxRequestBodyBytes = 1 << 20 // 1 MiB

// APIError is an RFC-style structured error response body.
type APIError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Detail  string `json:"detail,omitempty"`
	Details any    `json:"details,omitempty"`
}

// JSON writes status and encodes payload as application/json.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// Error writes a structured JSON error payload.
func Error(w http.ResponseWriter, status int, code, detail string) {
	JSON(w, status, APIError{
		Status: status,
		Code:   code,
		Title:  http.StatusText(status),
		Detail: detail,
	})
}

// ErrorWithDetails writes a structured JSON error payload with extra details.
func ErrorWithDetails(w http.ResponseWriter, status int, code, detail string, details any) {
	JSON(w, status, APIError{
		Status:  status,
		Code:    code,
		Title:   http.StatusText(status),
		Detail:  detail,
		Details: details,
	})
}

// DecodeJSON decodes a size-limited JSON request body into dst.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
