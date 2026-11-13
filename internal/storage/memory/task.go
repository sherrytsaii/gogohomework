// Package memory provides in-memory repository implementations.
// Each instance has independent data and IDs, with no persistence.
package memory

import (
	"cmp"
	"math"
	"slices"
	"sync"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

// TaskRepository stores tasks in process memory and is safe for concurrent use.
// Its zero value is ready for use. It must not be copied after first use.
type TaskRepository struct {
	mu     sync.RWMutex
	tasks  map[int64]task.Task
	lastID int64
}

var _ task.Repository = (*TaskRepository)(nil)

// NewTaskRepository returns an empty repository with IDs starting at 1.
func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
}

// List returns an independent, non-nil snapshot ordered by ascending ID.
func (r *TaskRepository) List() ([]task.Task, error) {
	r.mu.RLock()
	tasks := make([]task.Task, 0, len(r.tasks))
	for _, item := range r.tasks {
		tasks = append(tasks, item)
	}
	r.mu.RUnlock()

	slices.SortFunc(tasks, func(a, b task.Task) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return tasks, nil
}

// Create stores validated fields with the next ID; deleted IDs are not reused.
// It returns apperrors.ErrIDExhausted when no IDs remain.
func (r *TaskRepository) Create(name string, status task.Status) (task.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastID == math.MaxInt64 {
		return task.Task{}, apperrors.ErrIDExhausted
	}
	if r.tasks == nil {
		r.tasks = make(map[int64]task.Task)
	}
	r.lastID++
	created := task.Task{ID: r.lastID, Name: name, Status: status}
	r.tasks[created.ID] = created
	return created, nil
}

// Update replaces validated fields while preserving ID, or returns apperrors.ErrNotFound.
func (r *TaskRepository) Update(id int64, name string, status task.Status) (task.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[id]; !exists {
		return task.Task{}, apperrors.ErrNotFound
	}
	updated := task.Task{ID: id, Name: name, Status: status}
	r.tasks[id] = updated
	return updated, nil
}

// Delete removes a task or returns apperrors.ErrNotFound.
func (r *TaskRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[id]; !exists {
		return apperrors.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}
