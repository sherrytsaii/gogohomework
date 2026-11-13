package task

// Repository defines the storage operations required by the task service.
// Implementations must be safe for concurrent use.
type Repository interface {
	// List returns a non-nil snapshot ordered by ascending ID.
	List() ([]Task, error)
	// Create assigns a new positive ID without reusing previously assigned IDs.
	Create(name string, status Status) (Task, error)
	// Update preserves the ID and returns apperrors.ErrNotFound if the task does not exist.
	Update(id int64, name string, status Status) (Task, error)
	// Delete returns apperrors.ErrNotFound if the task does not exist.
	Delete(id int64) error
}
