// Package memory provides in-memory repository implementations.
package memory

import (
	"cmp"
	"math"
	"slices"
	"sync"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

// TaskRepository stores tasks for the lifetime of a process.
// Its zero value is ready for use. It must not be copied after first use.
type TaskRepository struct {
	mu     sync.RWMutex
	tasks  map[int64]task.Task
	lastID int64
}

var _ task.Repository = (*TaskRepository)(nil)

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
}

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

func (r *TaskRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[id]; !exists {
		return apperrors.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}
