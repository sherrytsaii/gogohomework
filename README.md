# gogohomework

A small REST API for creating, listing, updating, and deleting tasks, built with the Go standard library.

Requires Go **1.27.3 or later**, as specified in [go.mod](go.mod). Run the commands below from the repository root.

## Run

```sh
go run ./cmd/api
```

The server listens on `:8080` by default. Set `HTTP_ADDR` to override it, for example:

```sh
HTTP_ADDR=:9090 go run ./cmd/api
```

With the default address, try these requests in another terminal:

```sh
curl -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"name":"Write tests","status":0}'

curl http://localhost:8080/tasks
```

Task data and the ID counter are stored in memory and reset on restart. Logs are written to stdout as JSON. Stop the server with `Ctrl+C`.

## Checks

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
go build ./...
```

The formatting check (`gofmt -l .`) should produce no output.

## Documentation

See the [API GoDoc](internal/httpapi/doc.go) for routes, validation rules, and error responses, or use:

```sh
go doc ./internal/httpapi
go doc -all ./internal/task
```
