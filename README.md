# gogohomework

A small REST API for creating, listing, updating, and deleting tasks, built with the Go standard library.

Running locally requires Go **1.27.3 or later**, as specified in [go.mod](go.mod). Docker builds include Go. Run the commands below from the repository root.

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

## Docker

Install and start Docker with Linux container support. Build from the full repository; the API is compiled inside Docker with Go **1.27.3** and runs as a non-root user on Alpine.

```sh
docker build -t gogohomework .
docker run -d --rm --name gogohomework -p 127.0.0.1:8080:8080 gogohomework
curl http://localhost:8080/tasks
docker logs gogohomework
docker stop gogohomework
```

`-p 127.0.0.1:8080:8080` maps your computer's port 8080 to the container's port 8080. To use local port 9090, change it to `127.0.0.1:9090:8080`. `--rm` removes the container when it stops; the image remains available.

The Dockerfile supports `linux/arm64` and `linux/amd64`; each computer builds for its native architecture by default. Task data and IDs reset whenever the API process restarts, including inside a container.

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
