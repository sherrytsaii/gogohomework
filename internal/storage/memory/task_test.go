package memory

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

func TestTaskRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	var repository TaskRepository
	assertTasks(t, &repository, []task.Task{})

	first, err := repository.Create("Write tests", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.Create("Write tests", task.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	wantFirst := task.Task{ID: 1, Name: "Write tests", Status: task.StatusIncomplete}
	wantSecond := task.Task{ID: 2, Name: "Write tests", Status: task.StatusCompleted}
	if first != wantFirst || second != wantSecond {
		t.Fatalf("created tasks = %+v, %+v; want %+v, %+v", first, second, wantFirst, wantSecond)
	}
	assertTasks(t, &repository, []task.Task{wantFirst, wantSecond})

	updated, err := repository.Update(first.ID, "Review tests", task.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	wantUpdated := task.Task{ID: first.ID, Name: "Review tests", Status: task.StatusCompleted}
	if updated != wantUpdated {
		t.Errorf("updated task = %+v, want %+v", updated, wantUpdated)
	}
	assertTasks(t, &repository, []task.Task{wantUpdated, wantSecond})

	if err := repository.Delete(second.ID); err != nil {
		t.Fatal(err)
	}
	third, err := repository.Create("Commit changes", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	if third.ID != 3 {
		t.Errorf("ID after deleting the highest ID = %d, want 3", third.ID)
	}
	assertTasks(t, &repository, []task.Task{wantUpdated, third})

	for _, id := range []int64{first.ID, third.ID} {
		if err := repository.Delete(id); err != nil {
			t.Fatal(err)
		}
	}
	assertTasks(t, &repository, []task.Task{})
	fourth, err := repository.Create("Next task", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	if fourth.ID != 4 {
		t.Errorf("ID after deleting all tasks = %d, want 4", fourth.ID)
	}
}

func TestTaskRepositoryMissingTask(t *testing.T) {
	t.Parallel()

	repository := NewTaskRepository()
	item, err := repository.Create("Keep this task", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Update(99, "Missing", task.StatusCompleted); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("update error = %v, want %v", err, apperrors.ErrNotFound)
	}
	if err := repository.Delete(99); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("delete error = %v, want %v", err, apperrors.ErrNotFound)
	}
	assertTasks(t, repository, []task.Task{item})
	if err := repository.Delete(item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Update(item.ID, "Deleted", task.StatusCompleted); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("update deleted task error = %v, want %v", err, apperrors.ErrNotFound)
	}
	if err := repository.Delete(item.ID); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("delete deleted task error = %v, want %v", err, apperrors.ErrNotFound)
	}
	assertTasks(t, repository, []task.Task{})
}

func TestTaskRepositoryListSnapshot(t *testing.T) {
	t.Parallel()

	repository := NewTaskRepository()
	item, err := repository.Create("Original", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	snapshot[0].Name = "Changed by caller"
	snapshot[0].ID = 99
	assertTasks(t, repository, []task.Task{item})

	updated, err := repository.Update(item.ID, "Updated", task.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot[0].Name != "Changed by caller" || snapshot[0].ID != 99 {
		t.Error("repository update modified a previously returned snapshot")
	}
	assertTasks(t, repository, []task.Task{updated})
}

func TestTaskRepositoryInstancesAreIndependent(t *testing.T) {
	t.Parallel()

	first := NewTaskRepository()
	if _, err := first.Create("First repository", task.StatusIncomplete); err != nil {
		t.Fatal(err)
	}
	second := NewTaskRepository()
	assertTasks(t, second, []task.Task{})
	item, err := second.Create("Second repository", task.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 1 {
		t.Errorf("first ID in a new repository = %d, want 1", item.ID)
	}
	assertTasks(t, first, []task.Task{{ID: 1, Name: "First repository", Status: task.StatusIncomplete}})
}

func TestTaskRepositoryIDExhaustion(t *testing.T) {
	t.Parallel()

	// Arrange the boundary directly; allocating MaxInt64 tasks is impractical.
	repository := &TaskRepository{lastID: math.MaxInt64 - 1}
	last, err := repository.Create("Last ID", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	if last.ID != math.MaxInt64 {
		t.Fatalf("last ID = %d, want %d", last.ID, int64(math.MaxInt64))
	}
	if _, err := repository.Create("Overflow", task.StatusIncomplete); !errors.Is(err, apperrors.ErrIDExhausted) {
		t.Errorf("create error = %v, want %v", err, apperrors.ErrIDExhausted)
	}
	assertTasks(t, repository, []task.Task{last})
	if err := repository.Delete(last.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create("Reuse", task.StatusIncomplete); !errors.Is(err, apperrors.ErrIDExhausted) {
		t.Errorf("create after deletion error = %v, want %v", err, apperrors.ErrIDExhausted)
	}
	assertTasks(t, repository, []task.Task{})
}

func TestTaskRepositoryConcurrentCreate(t *testing.T) {
	t.Parallel()

	const count = 128
	repository := NewTaskRepository()
	created := make([]task.Task, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			created[i], errs[i] = repository.Create(fmt.Sprintf("Task %d", i), task.Status(i%2))
		}()
	}
	close(start)
	wg.Wait()

	seen := make(map[int64]task.Task, count)
	for i, item := range created {
		if errs[i] != nil {
			t.Fatalf("create %d: %v", i, errs[i])
		}
		if item.ID <= 0 || item.ID > count {
			t.Fatalf("ID = %d, want 1 through %d", item.ID, count)
		}
		if _, exists := seen[item.ID]; exists {
			t.Fatalf("duplicate ID %d", item.ID)
		}
		if item.Name != fmt.Sprintf("Task %d", i) || item.Status != task.Status(i%2) {
			t.Fatalf("create %d returned the wrong task: %+v", i, item)
		}
		seen[item.ID] = item
	}
	want := make([]task.Task, count)
	for i := range count {
		want[i] = seen[int64(i+1)]
	}
	assertTasks(t, repository, want)
}

func TestTaskRepositoryConcurrentAccess(t *testing.T) {
	t.Parallel()

	const workers = 32
	repository := NewTaskRepository()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			item, err := repository.Create(fmt.Sprintf("Task %d", i), task.StatusIncomplete)
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			for round := range 8 {
				updated, err := repository.Update(item.ID, item.Name, task.Status(round%2))
				if err != nil {
					t.Errorf("update: %v", err)
					return
				}
				if updated.ID != item.ID || updated.Name != item.Name || updated.Status != task.Status(round%2) {
					t.Errorf("unexpected update result: %+v", updated)
					return
				}
				snapshot, err := repository.List()
				if err != nil {
					t.Errorf("list: %v", err)
					return
				}
				for j := 1; j < len(snapshot); j++ {
					if snapshot[j-1].ID >= snapshot[j].ID {
						t.Error("snapshot contains unordered or duplicate IDs")
						return
					}
				}
			}
			if err := repository.Delete(item.ID); err != nil {
				t.Errorf("delete: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	assertTasks(t, repository, []task.Task{})
}

func TestTaskRepositoryConcurrentUpdateAndDelete(t *testing.T) {
	t.Parallel()

	const workers = 32
	repository := NewTaskRepository()
	item, err := repository.Create("Shared task", task.StatusIncomplete)
	if err != nil {
		t.Fatal(err)
	}
	deleteErrors := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range workers {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := repository.Update(item.ID, "Updated", task.StatusCompleted)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				t.Errorf("update: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			deleteErrors[i] = repository.Delete(item.ID)
		}()
	}
	close(start)
	wg.Wait()

	deleted := 0
	for _, err := range deleteErrors {
		if err == nil {
			deleted++
		} else if !errors.Is(err, apperrors.ErrNotFound) {
			t.Errorf("delete: %v", err)
		}
	}
	if deleted != 1 {
		t.Errorf("successful deletes = %d, want 1", deleted)
	}
	assertTasks(t, repository, []task.Task{})
}

func assertTasks(t *testing.T, repository *TaskRepository, want []task.Task) {
	t.Helper()
	got, err := repository.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
	}
}
