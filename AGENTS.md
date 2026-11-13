# Project Guidelines

## Tech Stack

- Go, using the version specified in `go.mod`.

## Coding Rules

- Follow idiomatic Go conventions and format code with `gofmt`.
- Write concise English GoDoc for packages and exported declarations, focusing on purpose, essential constraints, and error behavior.
- Avoid redundant comments and implementation walkthroughs; use `doc.go` for package overviews when helpful.
- Keep application entry points in `cmd/` and implementation packages in `internal/`.
- Keep packages focused, with clear responsibilities and dependency boundaries.
- Inject dependencies through constructors; avoid mutable global state and unnecessary abstractions.
- Handle errors explicitly, validate inputs, and protect shared state from concurrent access.
- Implement only the step requested by the user and wait for instructions before proceeding.

## Testing

- Write unit tests alongside each implementation change using the standard `testing` package.
- Cover expected behavior, boundary conditions, and error cases with deterministic, independent tests.
- Before every commit, complete the relevant unit tests and pass `go test ./...`, `go vet ./...`, and `go build ./...`.
- Use `gofmt -l .` as a formatting check; its output must be empty.
- Run `go test -race ./...` when changing concurrent behavior.
- Report failed or unavailable checks accurately; do not treat missing tests as successful validation.

## Code Review

- Review the diff for correctness, readability, concurrency safety, and compliance with the agreed requirements.
- Keep changes focused on the requested scope and exclude unrelated files or generated artifacts.
- Verify that tests cover the changed behavior and documentation reflects relevant changes.
- Check updated GoDoc with `go doc`.
- Present completed changes and validation results for user review; wait for explicit user approval before each commit.
- After approval and successful checks, commit cohesive changes using concise, descriptive messages and report the commit hash.
