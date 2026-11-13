package task_test

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

func TestServiceWriteFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label       string
		name        string
		status      task.Status
		wantName    string
		wantField   string
		wantMessage string
	}{
		{label: "incomplete", name: "Write tests", status: task.StatusIncomplete, wantName: "Write tests"},
		{label: "completed", name: "Write tests", status: task.StatusCompleted, wantName: "Write tests"},
		{label: "trim whitespace", name: " \tWrite tests\n", status: task.StatusIncomplete, wantName: "Write tests"},
		{label: "unicode whitespace", name: "\u3000完成 作業\u00a0", status: task.StatusCompleted, wantName: "完成 作業"},
		{label: "empty name", wantField: "name", wantMessage: "must not be blank"},
		{label: "blank name", name: " \t\n\u3000", wantField: "name", wantMessage: "must not be blank"},
		{label: "negative status", name: "Write tests", status: -1, wantField: "status", wantMessage: "must be 0 or 1"},
		{label: "unsupported status", name: "Write tests", status: 2, wantField: "status", wantMessage: "must be 0 or 1"},
	}
	operations := []struct {
		name string
		run  func(*task.Service, string, task.Status) (task.Task, error)
	}{
		{name: "create", run: (*task.Service).Create},
		{name: "update", run: func(s *task.Service, name string, status task.Status) (task.Task, error) {
			return s.Update(7, name, status)
		}},
	}

	for _, operation := range operations {
		for _, tt := range tests {
			t.Run(operation.name+"/"+tt.label, func(t *testing.T) {
				t.Parallel()
				repository := &stubRepository{}
				service := task.NewService(repository)
				got, err := operation.run(service, tt.name, tt.status)
				if tt.wantField != "" {
					assertValidationError(t, err, tt.wantField, tt.wantMessage)
					if repository.calls != 0 {
						t.Error("invalid input reached the repository")
					}
					if got != (task.Task{}) {
						t.Errorf("task = %+v, want zero value on error", got)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				want := task.Task{ID: 7, Name: tt.wantName, Status: tt.status}
				if got != want {
					t.Errorf("task = %+v, want %+v", got, want)
				}
				if repository.calls != 1 {
					t.Errorf("repository calls = %d, want 1", repository.calls)
				}
			})
		}
	}
}

func TestServiceRejectsInvalidID(t *testing.T) {
	t.Parallel()

	for _, id := range []int64{0, -1, math.MinInt64} {
		for _, operation := range []string{"update", "delete"} {
			t.Run(operation+"/"+strconv.FormatInt(id, 10), func(t *testing.T) {
				t.Parallel()
				repository := &stubRepository{}
				service := task.NewService(repository)
				var err error
				if operation == "update" {
					_, err = service.Update(id, "Write tests", task.StatusIncomplete)
				} else {
					err = service.Delete(id)
				}
				assertValidationError(t, err, "id", "must be positive")
				if repository.calls != 0 {
					t.Error("invalid ID reached the repository")
				}
			})
		}
	}
}

func TestServiceList(t *testing.T) {
	t.Parallel()

	want := []task.Task{{ID: 1, Name: "Write tests", Status: task.StatusCompleted}}
	repository := &stubRepository{tasks: want}
	got, err := task.NewService(repository).List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
	}
	if repository.calls != 1 {
		t.Errorf("repository calls = %d, want 1", repository.calls)
	}
}

func TestServiceDelete(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{}
	if err := task.NewService(repository).Delete(math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || repository.id != math.MaxInt64 {
		t.Errorf("delete calls = %d, ID = %d; want 1 call with ID %d", repository.calls, repository.id, int64(math.MaxInt64))
	}
}

func TestServiceRepositoryErrors(t *testing.T) {
	t.Parallel()

	storageErr := errors.New("storage unavailable")
	tests := []struct {
		name string
		err  error
		run  func(*task.Service) error
	}{
		{name: "list", err: storageErr, run: func(s *task.Service) error {
			_, err := s.List()
			return err
		}},
		{name: "create", err: storageErr, run: func(s *task.Service) error {
			_, err := s.Create("Write tests", task.StatusIncomplete)
			return err
		}},
		{name: "ID exhaustion", err: apperrors.ErrIDExhausted, run: func(s *task.Service) error {
			_, err := s.Create("Write tests", task.StatusIncomplete)
			return err
		}},
		{name: "update missing", err: apperrors.ErrNotFound, run: func(s *task.Service) error {
			_, err := s.Update(7, "Write tests", task.StatusCompleted)
			return err
		}},
		{name: "delete missing", err: apperrors.ErrNotFound, run: func(s *task.Service) error {
			return s.Delete(7)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repository := &stubRepository{err: tt.err}
			err := tt.run(task.NewService(repository))
			if !errors.Is(err, tt.err) {
				t.Errorf("error = %v, want wrapped %v", err, tt.err)
			}
			if repository.calls != 1 {
				t.Errorf("repository calls = %d, want 1", repository.calls)
			}
		})
	}
}

func assertValidationError(t *testing.T, err error, field, message string) {
	t.Helper()
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Fatalf("error = %v, want %v", err, apperrors.ErrInvalidInput)
	}
	var validationErr *apperrors.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *apperrors.ValidationError", err)
	}
	if validationErr.Field != field || validationErr.Message != message {
		t.Errorf("validation error = %+v, want field %q and message %q", validationErr, field, message)
	}
}

type stubRepository struct {
	tasks []task.Task
	err   error
	calls int
	id    int64
}

func (r *stubRepository) List() ([]task.Task, error) {
	r.calls++
	return r.tasks, r.err
}

func (r *stubRepository) Create(name string, status task.Status) (task.Task, error) {
	r.calls++
	return task.Task{ID: 7, Name: name, Status: status}, r.err
}

func (r *stubRepository) Update(id int64, name string, status task.Status) (task.Task, error) {
	r.calls++
	r.id = id
	return task.Task{ID: id, Name: name, Status: status}, r.err
}

func (r *stubRepository) Delete(id int64) error {
	r.calls++
	r.id = id
	return r.err
}
