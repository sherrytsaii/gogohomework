// Command api runs the task REST API with in-memory storage.
//
//	go run ./cmd/api
//
// HTTP_ADDR defaults to :8080. Logs use JSON on stdout. Data and IDs reset on
// restart. SIGINT and SIGTERM allow up to five seconds for graceful shutdown.
package main
