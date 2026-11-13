package httpapi

import (
	"log/slog"
	"net/http"
)

type handler struct {
	tasks  TaskService
	logger *slog.Logger
}

// NewRouter registers the task endpoints, JSON errors, and access logging.
// tasks must support concurrent use. A nil logger selects slog.Default.
func NewRouter(tasks TaskService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handler{tasks: tasks, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", h.listTasks)
	mux.HandleFunc("POST /tasks", h.createTask)
	mux.HandleFunc("PUT /tasks/{id}", h.updateTask)
	mux.HandleFunc("DELETE /tasks/{id}", h.deleteTask)
	mux.HandleFunc("/tasks", h.methodNotAllowed("GET, HEAD, POST"))
	mux.HandleFunc("/tasks/{id}", h.methodNotAllowed("DELETE, PUT"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h.writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	})
	return accessLog(mux, logger)
}

func (h *handler) methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		h.writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}
