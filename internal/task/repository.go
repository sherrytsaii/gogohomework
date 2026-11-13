package task

// Repository stores validated tasks and must be safe for concurrent use.
type Repository interface {
	// List returns an independent, non-nil snapshot ordered by ascending ID.
	List() ([]Task, error)
	// Create atomically assigns a positive ID without reusing deleted IDs.
	Create(name string, status Status) (Task, error)
	// Update replaces the fields, preserving ID, or returns apperrors.ErrNotFound.
	Update(id int64, name string, status Status) (Task, error)
	// Delete removes a task or returns apperrors.ErrNotFound.
	Delete(id int64) error
}
