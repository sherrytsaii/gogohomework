package task

import (
	"fmt"
	"strings"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
)

// Service validates task operations before accessing the repository.
// Errors retain their identity for errors.Is and errors.As.
type Service struct {
	repository Repository
}

// NewService returns a service backed by repository, which must be non-nil.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// List returns a non-nil task snapshot ordered by ascending ID.
func (s *Service) List() ([]Task, error) {
	tasks, err := s.repository.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

// Create trims name and creates a task with a repository-assigned ID.
// Blank names and invalid statuses return an apperrors.ValidationError.
func (s *Service) Create(name string, status Status) (Task, error) {
	name, err := validateFields(name, status)
	if err != nil {
		return Task{}, err
	}

	created, err := s.repository.Create(name, status)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return created, nil
}

// Update replaces name and status while preserving ID. Invalid input returns an
// apperrors.ValidationError; a missing task returns a wrapped apperrors.ErrNotFound.
func (s *Service) Update(id int64, name string, status Status) (Task, error) {
	if id <= 0 {
		return Task{}, &apperrors.ValidationError{Field: "id", Message: "must be positive"}
	}
	name, err := validateFields(name, status)
	if err != nil {
		return Task{}, err
	}

	updated, err := s.repository.Update(id, name, status)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	return updated, nil
}

// Delete removes a task. Invalid IDs return an apperrors.ValidationError;
// missing tasks return a wrapped apperrors.ErrNotFound.
func (s *Service) Delete(id int64) error {
	if id <= 0 {
		return &apperrors.ValidationError{Field: "id", Message: "must be positive"}
	}
	if err := s.repository.Delete(id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func validateFields(name string, status Status) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &apperrors.ValidationError{Field: "name", Message: "must not be blank"}
	}
	if status != StatusIncomplete && status != StatusCompleted {
		return "", &apperrors.ValidationError{Field: "status", Message: "must be 0 or 1"}
	}
	return name, nil
}
