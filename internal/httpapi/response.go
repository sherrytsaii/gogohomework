package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		message := "invalid request"
		var validationErr *apperrors.ValidationError
		if errors.As(err, &validationErr) {
			message = validationErr.Error()
		}
		h.writeError(w, r, http.StatusBadRequest, "invalid_request", message)
	case errors.Is(err, apperrors.ErrNotFound):
		h.writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	default:
		h.logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	h.writeJSON(w, r, status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}

func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "encode response", "error", err)
		status = http.StatusInternalServerError
		body = []byte(`{"error":{"code":"internal_error","message":"internal server error"}}`)
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}
