// Package httpapi serves the task REST API with JSON responses and access logs.
//
// Routes:
//
//	GET    /tasks      200: task list, or [] when empty
//	POST   /tasks      201: created task
//	PUT    /tasks/{id} 200: updated task
//	DELETE /tasks/{id} 204: no body
//
// POST and PUT require application/json with non-null name and status fields.
// Names are trimmed and must not be blank; status is 0 (incomplete) or 1
// (completed). Bodies are limited to 1 MiB; unknown fields, including id, are
// rejected. Route IDs must be positive int64 values. PUT replaces both fields.
//
// Errors use the envelope {"error":{"code":"...","message":"..."}}:
//
//   - 400: invalid input; 404: missing resource; 405: unsupported method.
//   - 413: oversized body; 415: unsupported media type; 500: internal error.
//
// Access logs record method, path, status, and duration_ms after each request.
// Internal error details are logged but not returned to clients.
package httpapi
