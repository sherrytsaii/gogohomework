package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// accessLog records method, path, status, and duration_ms after each request.
// Panics propagate after an aborted log entry. next and logger must be non-nil.
func accessLog(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		method, path := r.Method, r.URL.Path
		response := &statusWriter{ResponseWriter: w}
		completed := false
		defer func() {
			message := "HTTP request completed"
			status := response.status
			if !completed {
				// Preserve the panic for net/http to handle. Status 0 means no
				// final response header was written before the request aborted.
				message = "HTTP request aborted"
			} else if status == 0 {
				status = http.StatusOK
			}
			logger.InfoContext(r.Context(), message,
				"method", method,
				"path", path,
				"status", status,
				"duration_ms", float64(time.Since(started))/float64(time.Millisecond),
			)
		}()
		next.ServeHTTP(response, r)
		completed = true
	})
}

// statusWriter tracks the first final response status for one request.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader forwards headers and records the first terminal status.
func (w *statusWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	// Informational responses do not replace the final status. A protocol
	// switch is terminal, matching net/http's handling of status 101.
	if w.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		w.status = status
	}
}

// Write preserves the underlying write result and tracks an implicit 200 status.
func (w *statusWriter) Write(body []byte) (int, error) {
	n, err := w.ResponseWriter.Write(body)
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return n, err
}

// FlushError tracks the implicit 200 response sent by a supported flush.
func (w *statusWriter) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if w.status == 0 && !errors.Is(err, http.ErrNotSupported) {
		w.status = http.StatusOK
	}
	return err
}

// Unwrap preserves access to the underlying writer through ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
