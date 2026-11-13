package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

const maxRequestBytes = 1 << 20

// TaskService defines the operations used by the task HTTP handlers.
// Implementations validate task fields, support concurrent use, and preserve
// shared apperrors categories. A *task.Service satisfies this interface.
type TaskService interface {
	// List returns a snapshot ordered by ascending ID.
	List() ([]task.Task, error)
	// Create validates the fields and returns a task with a server-assigned ID.
	Create(name string, status task.Status) (task.Task, error)
	// Update replaces the fields of an existing task while preserving its ID.
	Update(id int64, name string, status task.Status) (task.Task, error)
	// Delete removes an existing task or returns a shared not-found error.
	Delete(id int64) error
}

// taskInput uses pointers to distinguish missing or null fields from zero values.
type taskInput struct {
	Name   *string      `json:"name"`
	Status *task.Status `json:"status"`
}

func (h *handler) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.tasks.List()
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	if tasks == nil {
		tasks = []task.Task{}
	}
	h.writeJSON(w, r, http.StatusOK, tasks)
}

func (h *handler) createTask(w http.ResponseWriter, r *http.Request) {
	input, ok := h.readTaskInput(w, r)
	if !ok {
		return
	}
	created, err := h.tasks.Create(*input.Name, *input.Status)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusCreated, created)
}

func (h *handler) updateTask(w http.ResponseWriter, r *http.Request) {
	id, err := taskID(r)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	input, ok := h.readTaskInput(w, r)
	if !ok {
		return
	}
	updated, err := h.tasks.Update(id, *input.Name, *input.Status)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, updated)
}

func (h *handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := taskID(r)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	if err := h.tasks.Delete(id); err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readTaskInput checks request format and required fields.
// On false, the error response is already written and callers must return.
func (h *handler) readTaskInput(w http.ResponseWriter, r *http.Request) (taskInput, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		h.writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return taskInput{}, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input taskInput
	if err := decoder.Decode(&input); err != nil {
		h.writeDecodeError(w, r, err)
		return taskInput{}, false
	}
	// Require EOF so a second JSON value or trailing garbage cannot be ignored.
	if err := decoder.Decode(new(any)); err != io.EOF {
		h.writeDecodeError(w, r, err)
		return taskInput{}, false
	}
	if input.Name == nil {
		h.writeServiceError(w, r, &apperrors.ValidationError{Field: "name", Message: "is required"})
		return taskInput{}, false
	}
	if input.Status == nil {
		h.writeServiceError(w, r, &apperrors.ValidationError{Field: "status", Message: "is required"})
		return taskInput{}, false
	}
	return input, true
}

func (h *handler) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		h.writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds 1 MiB")
		return
	}
	h.writeError(w, r, http.StatusBadRequest, "invalid_request", "body must contain one JSON object with only name and status")
}

func taskID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, &apperrors.ValidationError{Field: "id", Message: "must be a positive integer"}
	}
	return id, nil
}
