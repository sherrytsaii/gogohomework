package task

import (
	"fmt"
	"strings"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
)

// Service validates task operations before accessing the repository.
type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List() ([]Task, error) {
	tasks, err := s.repository.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

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
