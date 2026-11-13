// Package task defines tasks and their application logic.
package task

// Status describes whether a task is complete.
type Status int

const (
	StatusIncomplete Status = iota
	StatusCompleted
)

// Task is a task identified by a server-assigned ID.
type Task struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
}
