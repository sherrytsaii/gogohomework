package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONEncodingFailure(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	h := &handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	w := httptest.NewRecorder()
	h.writeJSON(w, httptest.NewRequest(http.MethodGet, "/tasks", nil), http.StatusOK, make(chan int))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"code":"internal_error"`) {
		t.Errorf("response = %d, %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "unsupported type") || !strings.Contains(logs.String(), "unsupported type") {
		t.Error("encoding failure must be logged without exposing details in the response")
	}
}

func TestWriteJSONWriteFailure(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	h := &handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	w := &failingWriter{header: make(http.Header)}
	h.writeJSON(w, httptest.NewRequest(http.MethodGet, "/tasks", nil), http.StatusOK, []string{})
	if !strings.Contains(logs.String(), "write response") || !strings.Contains(logs.String(), "client disconnected") {
		t.Errorf("write failure was not logged: %s", logs.String())
	}
}

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header { return w.header }
func (w *failingWriter) WriteHeader(int)     {}
func (w *failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
}
