// Package task defines task models, validation, and service operations.
package task

// Status describes whether a task is complete.
type Status int

const (
	// StatusIncomplete means the task is unfinished (0).
	StatusIncomplete Status = iota
	// StatusCompleted means the task is finished (1).
	StatusCompleted
)

// Task is a task identified by a server-assigned ID.
type Task struct {
	// ID is a repository-assigned positive identifier.
	ID int64 `json:"id"`
	// Name is trimmed and validated by Service.
	Name string `json:"name"`
	// Status is StatusIncomplete or StatusCompleted.
	Status Status `json:"status"`
}
