package httpapi_test

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
	"strconv"
	"strings"
	"testing"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/httpapi"
	"github.com/sherrytsaii/gogohomework/internal/storage/memory"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

func TestTaskLifecycle(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter()
	assertTaskList(t, router, []task.Task{})

	response := request(router, http.MethodPost, "/tasks", `{"name":"  First  ","status":0}`, "application/json")
	first := decodeJSON[task.Task](t, response, http.StatusCreated)
	if first != (task.Task{ID: 1, Name: "First", Status: task.StatusIncomplete}) {
		t.Fatalf("first task = %+v", first)
	}
	response = request(router, http.MethodPost, "/tasks", `{"name":"Second","status":1}`, "application/json; charset=utf-8")
	second := decodeJSON[task.Task](t, response, http.StatusCreated)
	if second != (task.Task{ID: 2, Name: "Second", Status: task.StatusCompleted}) {
		t.Fatalf("second task = %+v", second)
	}
	assertTaskList(t, router, []task.Task{first, second})

	for range 2 {
		response = request(router, http.MethodPut, "/tasks/1", `{"name":" Updated ","status":1}`, "application/json")
		updated := decodeJSON[task.Task](t, response, http.StatusOK)
		if updated != (task.Task{ID: 1, Name: "Updated", Status: task.StatusCompleted}) {
			t.Fatalf("updated task = %+v", updated)
		}
		first = updated
	}
	assertTaskList(t, router, []task.Task{first, second})

	response = request(router, http.MethodDelete, "/tasks/2", "", "")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || response.Header().Get("Content-Type") != "" {
		t.Fatalf("delete response = %d, %q, %q", response.Code, response.Body.String(), response.Header().Get("Content-Type"))
	}
	assertError(t, request(router, http.MethodDelete, "/tasks/2", "", ""), http.StatusNotFound, "not_found")
	assertError(t, request(router, http.MethodPut, "/tasks/2", `{"name":"Deleted","status":0}`, "application/json"), http.StatusNotFound, "not_found")
	assertTaskList(t, router, []task.Task{first})

	response = request(router, http.MethodPost, "/tasks", `{"name":"Third","status":0}`, "application/json")
	third := decodeJSON[task.Task](t, response, http.StatusCreated)
	if third.ID != 3 {
		t.Errorf("ID after deletion = %d, want 3", third.ID)
	}
	assertTaskList(t, router, []task.Task{first, third})
}

func TestInvalidTaskInputDoesNotMutateStorage(t *testing.T) {
	t.Parallel()
	validBody := `{"name":"Task","status":0}`
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
		code        string
	}{
		{name: "empty", body: ""},
		{name: "whitespace", body: " \n\t"},
		{name: "malformed", body: `{"name":`},
		{name: "null", body: `null`},
		{name: "array", body: `[]`},
		{name: "string", body: `"task"`},
		{name: "empty object", body: `{}`},
		{name: "missing name", body: `{"status":0}`},
		{name: "missing status", body: `{"name":"Task"}`},
		{name: "null name", body: `{"name":null,"status":0}`},
		{name: "null status", body: `{"name":"Task","status":null}`},
		{name: "blank name", body: `{"name":" \t ","status":0}`},
		{name: "non-string name", body: `{"name":123,"status":0}`},
		{name: "string status", body: `{"name":"Task","status":"0"}`},
		{name: "boolean status", body: `{"name":"Task","status":false}`},
		{name: "decimal status", body: `{"name":"Task","status":1.0}`},
		{name: "negative status", body: `{"name":"Task","status":-1}`},
		{name: "unsupported status", body: `{"name":"Task","status":2}`},
		{name: "overflow status", body: `{"name":"Task","status":999999999999999999999999}`},
		{name: "unknown field", body: `{"name":"Task","status":0,"extra":true}`},
		{name: "client ID", body: `{"id":99,"name":"Task","status":0}`},
		{name: "two values", body: validBody + `{}`},
		{name: "trailing null", body: validBody + ` null`},
		{name: "trailing garbage", body: validBody + ` garbage`},
		{name: "oversized name", body: `{"name":"` + strings.Repeat("x", 1<<20) + `","status":0}`, status: http.StatusRequestEntityTooLarge, code: "request_too_large"},
		{name: "oversized trailing whitespace", body: validBody + strings.Repeat(" ", (1<<20)+1-len(validBody)), status: http.StatusRequestEntityTooLarge, code: "request_too_large"},
		{name: "text content type", body: validBody, contentType: "text/plain", status: http.StatusUnsupportedMediaType, code: "unsupported_media_type"},
		{name: "malformed content type", body: validBody, contentType: "application/json; charset", status: http.StatusUnsupportedMediaType, code: "unsupported_media_type"},
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, tt := range tests {
			t.Run(method+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				router, service := newTestRouter()
				path := "/tasks"
				want := []task.Task{}
				if method == http.MethodPut {
					original, err := service.Create("Original", task.StatusCompleted)
					if err != nil {
						t.Fatal(err)
					}
					want = append(want, original)
					path = "/tasks/1"
				}
				contentType := tt.contentType
				if contentType == "" {
					contentType = "application/json"
				}
				status, code := tt.status, tt.code
				if status == 0 {
					status, code = http.StatusBadRequest, "invalid_request"
				}
				assertError(t, request(router, method, path, tt.body, contentType), status, code)
				assertTaskList(t, router, want)
			})
		}
	}
}

func TestMissingContentType(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter()
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		path := "/tasks"
		if method == http.MethodPut {
			path += "/1"
		}
		assertError(t, request(router, method, path, `{"name":"Task","status":0}`, ""), http.StatusUnsupportedMediaType, "unsupported_media_type")
	}
}

func TestRequestBodyAtLimit(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter()
	body := `{"name":"Task","status":0}`
	body += strings.Repeat(" ", (1<<20)-len(body))
	created := decodeJSON[task.Task](t, request(router, http.MethodPost, "/tasks", body, "application/json"), http.StatusCreated)
	if created.Name != "Task" || created.Status != task.StatusIncomplete {
		t.Errorf("task = %+v", created)
	}
}

func TestTaskID(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		for _, id := range []string{"0", "-1", "abc", "1.5", "9223372036854775808"} {
			t.Run(method+"/"+id, func(t *testing.T) {
				t.Parallel()
				router, _ := newTestRouter()
				response := request(router, method, "/tasks/"+id, `{"name":"Task","status":0}`, "application/json")
				body := assertError(t, response, http.StatusBadRequest, "invalid_request")
				if body.Error.Message != "id: must be a positive integer" {
					t.Errorf("message = %q", body.Error.Message)
				}
			})
		}
		router, _ := newTestRouter()
		response := request(router, method, "/tasks/9223372036854775807", `{"name":"Task","status":0}`, "application/json")
		assertError(t, response, http.StatusNotFound, "not_found")
	}
}

func TestRoutingErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method string
		path   string
		status int
		code   string
		allow  string
	}{
		{http.MethodGet, "/unknown", http.StatusNotFound, "not_found", ""},
		{http.MethodPost, "/unknown", http.StatusNotFound, "not_found", ""},
		{http.MethodGet, "/tasks/", http.StatusNotFound, "not_found", ""},
		{http.MethodGet, "/tasks/1/extra", http.StatusNotFound, "not_found", ""},
		{http.MethodPut, "/tasks", http.StatusMethodNotAllowed, "method_not_allowed", "GET, HEAD, POST"},
		{http.MethodDelete, "/tasks", http.StatusMethodNotAllowed, "method_not_allowed", "GET, HEAD, POST"},
		{http.MethodOptions, "/tasks", http.StatusMethodNotAllowed, "method_not_allowed", "GET, HEAD, POST"},
		{http.MethodGet, "/tasks/1", http.StatusMethodNotAllowed, "method_not_allowed", "DELETE, PUT"},
		{http.MethodPost, "/tasks/1", http.StatusMethodNotAllowed, "method_not_allowed", "DELETE, PUT"},
		{http.MethodPatch, "/tasks/1", http.StatusMethodNotAllowed, "method_not_allowed", "DELETE, PUT"},
	}
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			t.Parallel()
			router, _ := newTestRouter()
			response := request(router, tt.method, tt.path, "", "")
			assertError(t, response, tt.status, tt.code)
			if got := response.Header().Get("Allow"); got != tt.allow {
				t.Errorf("Allow = %q, want %q", got, tt.allow)
			}
		})
	}
}

func TestHead(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter()
	get := request(router, http.MethodGet, "/tasks", "", "")
	head := request(router, http.MethodHead, "/tasks", "", "")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d, %q", head.Code, head.Body.String())
	}
	if head.Header().Get("Content-Type") != "application/json" || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Errorf("HEAD headers = %v", head.Header())
	}
}

func TestServiceErrors(t *testing.T) {
	t.Parallel()
	internalErr := errors.New("private storage failure")
	tests := []struct {
		name    string
		method  string
		path    string
		err     error
		status  int
		code    string
		message string
	}{
		{"list failure", http.MethodGet, "/tasks", internalErr, http.StatusInternalServerError, "internal_error", "internal server error"},
		{"create failure", http.MethodPost, "/tasks", internalErr, http.StatusInternalServerError, "internal_error", "internal server error"},
		{"update failure", http.MethodPut, "/tasks/1", internalErr, http.StatusInternalServerError, "internal_error", "internal server error"},
		{"delete failure", http.MethodDelete, "/tasks/1", internalErr, http.StatusInternalServerError, "internal_error", "internal server error"},
		{"ID exhaustion", http.MethodPost, "/tasks", apperrors.ErrIDExhausted, http.StatusInternalServerError, "internal_error", "internal server error"},
		{"wrapped missing", http.MethodPut, "/tasks/1", fmt.Errorf("private details: %w", apperrors.ErrNotFound), http.StatusNotFound, "not_found", "resource not found"},
		{"wrapped invalid input", http.MethodPost, "/tasks", fmt.Errorf("private details: %w", apperrors.ErrInvalidInput), http.StatusBadRequest, "invalid_request", "invalid request"},
		{"wrapped validation", http.MethodPost, "/tasks", fmt.Errorf("private details: %w", &apperrors.ValidationError{Field: "name", Message: "must not be blank"}), http.StatusBadRequest, "invalid_request", "name: must not be blank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			service := &failingTaskService{err: tt.err}
			router := httpapi.NewRouter(service, slog.New(slog.NewJSONHandler(&logs, nil)))
			response := request(router, tt.method, tt.path, `{"name":"Task","status":0}`, "application/json")
			body := assertError(t, response, tt.status, tt.code)
			if body.Error.Message != tt.message {
				t.Errorf("message = %q, want %q", body.Error.Message, tt.message)
			}
			if service.calls != 1 {
				t.Errorf("service calls = %d, want 1", service.calls)
			}
			if tt.status == http.StatusInternalServerError && !strings.Contains(logs.String(), tt.err.Error()) {
				t.Error("internal error details were not logged")
			}
		})
	}
}

func TestNilTaskList(t *testing.T) {
	t.Parallel()
	router := httpapi.NewRouter(&failingTaskService{}, nil)
	assertTaskList(t, router, []task.Task{})
}

func newTestRouter() (http.Handler, *task.Service) {
	service := task.NewService(memory.NewTaskRepository())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewRouter(service, logger), service
}

func request(router http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func decodeJSON[T any](t *testing.T, response *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	return result
}

func assertTaskList(t *testing.T, router http.Handler, want []task.Task) {
	t.Helper()
	got := decodeJSON[[]task.Task](t, request(router, http.MethodGet, "/tasks", "", ""), http.StatusOK)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
	}
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) errorEnvelope {
	t.Helper()
	body := decodeJSON[errorEnvelope](t, response, status)
	if body.Error.Code != code || body.Error.Message == "" {
		t.Errorf("error = %+v, want code %q with a message", body.Error, code)
	}
	return body
}

type failingTaskService struct {
	err   error
	calls int
}

func (s *failingTaskService) List() ([]task.Task, error) {
	s.calls++
	return nil, s.err
}

func (s *failingTaskService) Create(string, task.Status) (task.Task, error) {
	s.calls++
	return task.Task{}, s.err
}

func (s *failingTaskService) Update(int64, string, task.Status) (task.Task, error) {
	s.calls++
	return task.Task{}, s.err
}

func (s *failingTaskService) Delete(int64) error {
	s.calls++
	return s.err
}
