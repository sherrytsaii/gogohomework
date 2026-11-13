package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sherrytsaii/gogohomework/internal/storage/memory"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

func TestAccessLogCompletion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		handle http.HandlerFunc
	}{
		{"empty handler", http.StatusOK, func(http.ResponseWriter, *http.Request) {}},
		{"implicit status", http.StatusOK, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "response body")
		}},
		{"created", http.StatusCreated, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":1}`)
		}},
		{"no content", http.StatusNoContent, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}},
		{"client error", http.StatusBadRequest, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "invalid input", http.StatusBadRequest)
		}},
		{"server error", http.StatusInternalServerError, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}},
		{"first final status wins", http.StatusCreated, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusInternalServerError)
		}},
		{"flush commits status", http.StatusOK, func(w http.ResponseWriter, _ *http.Request) {
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusCreated)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			plain, logged := httptest.NewRecorder(), httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/tasks?token=private-query", strings.NewReader("private-body"))
			r.Header.Set("Authorization", "private-header")
			tt.handle.ServeHTTP(plain, r)
			accessLog(tt.handle, logger).ServeHTTP(logged, r)
			if logged.Code != plain.Code || logged.Body.String() != plain.Body.String() || !reflect.DeepEqual(logged.Header(), plain.Header()) || logged.Flushed != plain.Flushed {
				t.Errorf("middleware changed the response: plain=%+v, logged=%+v", plain, logged)
			}
			entries := accessEntries(t, logs.Bytes())
			if len(entries) != 1 {
				t.Fatalf("access log count = %d, want 1", len(entries))
			}
			assertAccessEntry(t, entries[0], "HTTP request completed", http.MethodPost, "/tasks", tt.status)
			if strings.Contains(logs.String(), "private-") {
				t.Error("access log included query, header, or body content")
			}
		})
	}
}

func TestAccessLogWaitsForHandler(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	var handlerDuration time.Duration
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started := time.Now()
		if logs.Len() != 0 {
			t.Error("access log was written before the handler completed")
		}
		w.WriteHeader(http.StatusAccepted)
		handlerDuration = time.Since(started)
	})
	accessLog(next, logger).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/work", nil))
	entries := accessEntries(t, logs.Bytes())
	if len(entries) != 1 {
		t.Fatalf("access log count = %d, want 1", len(entries))
	}
	assertAccessEntry(t, entries[0], "HTTP request completed", http.MethodGet, "/work", http.StatusAccepted)
	if *entries[0].DurationMS < float64(handlerDuration)/float64(time.Millisecond) {
		t.Error("logged duration does not include the handler execution")
	}
}

func TestAccessLogPanic(t *testing.T) {
	t.Parallel()
	for _, status := range []int{0, http.StatusAccepted} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			wantPanic := errors.New("handler panic")
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if status != 0 {
					w.WriteHeader(status)
				}
				panic(wantPanic)
			})
			func() {
				defer func() {
					if got := recover(); got != wantPanic {
						t.Errorf("panic = %v, want original panic %v", got, wantPanic)
					}
				}()
				accessLog(next, logger).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))
				t.Error("middleware swallowed the panic")
			}()
			entries := accessEntries(t, logs.Bytes())
			if len(entries) != 1 {
				t.Fatalf("access log count = %d, want 1", len(entries))
			}
			assertAccessEntry(t, entries[0], "HTTP request aborted", http.MethodGet, "/panic", status)
		})
	}
}

func TestStatusWriterInformationalHeaders(t *testing.T) {
	t.Parallel()
	for _, final := range []int{http.StatusCreated, http.StatusSwitchingProtocols} {
		t.Run(fmt.Sprint(final), func(t *testing.T) {
			t.Parallel()
			underlying := &headerWriter{header: make(http.Header)}
			writer := &statusWriter{ResponseWriter: underlying}
			writer.WriteHeader(http.StatusEarlyHints)
			writer.WriteHeader(http.StatusContinue)
			if writer.status != 0 {
				t.Error("informational status was recorded as the final status")
			}
			writer.WriteHeader(final)
			writer.WriteHeader(http.StatusInternalServerError)
			if writer.status != final {
				t.Errorf("status = %d, want %d", writer.status, final)
			}
			want := []int{http.StatusEarlyHints, http.StatusContinue, final, http.StatusInternalServerError}
			if !reflect.DeepEqual(underlying.statuses, want) {
				t.Errorf("forwarded headers = %v, want %v", underlying.statuses, want)
			}
		})
	}
}

func TestStatusWriterUnsupportedFlush(t *testing.T) {
	t.Parallel()
	underlying := &headerWriter{header: make(http.Header)}
	writer := &statusWriter{ResponseWriter: underlying}
	if err := http.NewResponseController(writer).Flush(); !errors.Is(err, http.ErrNotSupported) {
		t.Errorf("flush error = %v, want ErrNotSupported", err)
	}
	if writer.status != 0 || len(underlying.statuses) != 0 {
		t.Error("unsupported flush committed response headers")
	}
	writer.WriteHeader(http.StatusCreated)
	if writer.status != http.StatusCreated {
		t.Errorf("status = %d, want 201", writer.status)
	}
}

func TestStatusWriterPreservesWriteError(t *testing.T) {
	t.Parallel()
	writer := &statusWriter{ResponseWriter: &failingWriter{header: make(http.Header)}}
	n, err := writer.Write([]byte("body"))
	if n != 0 || err == nil || err.Error() != "client disconnected" {
		t.Errorf("write returned %d, %v", n, err)
	}
	if writer.status != http.StatusOK {
		t.Errorf("implicit status = %d, want 200", writer.status)
	}
}

func TestStatusWriterResponseController(t *testing.T) {
	t.Parallel()
	underlying := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	writer := &statusWriter{ResponseWriter: underlying}
	deadline := time.Unix(123, 0)
	if err := http.NewResponseController(writer).SetWriteDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if underlying.deadline != deadline {
		t.Errorf("forwarded deadline = %v, want %v", underlying.deadline, deadline)
	}
}

func TestAccessLogConcurrentRequests(t *testing.T) {
	t.Parallel()
	const count = 64
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	middleware := accessLog(next, logger)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			method := http.MethodGet
			if i%2 != 0 {
				method = http.MethodPost
			}
			middleware.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, fmt.Sprintf("/requests/%d", i), nil))
		}()
	}
	wg.Wait()
	entries := accessEntries(t, logs.Bytes())
	if len(entries) != count {
		t.Fatalf("access log count = %d, want %d", len(entries), count)
	}
	byPath := make(map[string]accessEntry, count)
	for _, entry := range entries {
		if _, exists := byPath[entry.Path]; exists {
			t.Fatalf("duplicate request log for %s", entry.Path)
		}
		byPath[entry.Path] = entry
	}
	for i := range count {
		method, status := http.MethodGet, http.StatusOK
		if i%2 != 0 {
			method, status = http.MethodPost, http.StatusCreated
		}
		path := fmt.Sprintf("/requests/%d", i)
		assertAccessEntry(t, byPath[path], "HTTP request completed", method, path, status)
	}
}

func TestRouterAccessLogs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		status      int
	}{
		{"list", http.MethodGet, "/tasks", "", "", http.StatusOK},
		{"create", http.MethodPost, "/tasks", `{"name":"New","status":0}`, "application/json", http.StatusCreated},
		{"update", http.MethodPut, "/tasks/1", `{"name":"Updated","status":1}`, "application/json", http.StatusOK},
		{"delete", http.MethodDelete, "/tasks/1", "", "", http.StatusNoContent},
		{"head", http.MethodHead, "/tasks", "", "", http.StatusOK},
		{"invalid input", http.MethodPost, "/tasks", `{}`, "application/json", http.StatusBadRequest},
		{"unknown route", http.MethodGet, "/missing", "", "", http.StatusNotFound},
		{"method", http.MethodPatch, "/tasks", "", "", http.StatusMethodNotAllowed},
		{"content type", http.MethodPost, "/tasks", `{}`, "text/plain", http.StatusUnsupportedMediaType},
		{"body limit", http.MethodPost, "/tasks", `{"name":"` + strings.Repeat("x", 1<<20) + `","status":0}`, "application/json", http.StatusRequestEntityTooLarge},
		{"redirect", http.MethodGet, "/other/../tasks", "", "", http.StatusTemporaryRedirect},
		{"internal error", http.MethodGet, "/tasks", "", "", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			service := task.NewService(memory.NewTaskRepository())
			if _, err := service.Create("Original", task.StatusIncomplete); err != nil {
				t.Fatal(err)
			}
			var tasks TaskService = service
			if tt.status == http.StatusInternalServerError {
				tasks = &listErrorService{Service: service}
			}
			router := NewRouter(tasks, logger)
			r := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("response status = %d, want %d: %s", w.Code, tt.status, w.Body.String())
			}
			entries := accessEntries(t, logs.Bytes())
			if len(entries) != 1 {
				t.Fatalf("access log count = %d, want 1", len(entries))
			}
			assertAccessEntry(t, entries[0], "HTTP request completed", tt.method, tt.path, tt.status)
		})
	}
}

type accessEntry struct {
	Level      string   `json:"level"`
	Message    string   `json:"msg"`
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Status     int      `json:"status"`
	DurationMS *float64 `json:"duration_ms"`
}

func accessEntries(t *testing.T, data []byte) []accessEntry {
	t.Helper()
	var entries []accessEntry
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var entry accessEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			return entries
		} else if err != nil {
			t.Fatalf("invalid JSON log: %v", err)
		}
		if entry.Message == "HTTP request completed" || entry.Message == "HTTP request aborted" {
			entries = append(entries, entry)
		}
	}
}

func assertAccessEntry(t *testing.T, entry accessEntry, message, method, path string, status int) {
	t.Helper()
	if entry.Level != "INFO" || entry.Message != message || entry.Method != method || entry.Path != path || entry.Status != status {
		t.Errorf("access log = %+v, want INFO %q %s %s %d", entry, message, method, path, status)
	}
	if entry.DurationMS == nil || *entry.DurationMS < 0 {
		t.Fatal("access log must contain a non-negative duration_ms")
	}
}

type headerWriter struct {
	header   http.Header
	statuses []int
}

func (w *headerWriter) Header() http.Header         { return w.header }
func (w *headerWriter) WriteHeader(status int)      { w.statuses = append(w.statuses, status) }
func (w *headerWriter) Write(p []byte) (int, error) { return len(p), nil }

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

type listErrorService struct {
	*task.Service
}

func (s *listErrorService) List() ([]task.Task, error) {
	return nil, errors.New("storage unavailable")
}
