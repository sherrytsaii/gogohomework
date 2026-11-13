package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	t.Parallel()
	for _, address := range []string{"", "127.0.0.1:9000"} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			server := newServer(address, testLogger())
			want := address
			if want == "" {
				want = ":8080"
			}
			if server.Addr != want {
				t.Errorf("address = %q, want %q", server.Addr, want)
			}
			if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
				t.Error("server timeouts must be configured")
			}
			request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"name":"Task","status":0}`))
			request.Header.Set("Content-Type", "application/json")
			created := httptest.NewRecorder()
			server.Handler.ServeHTTP(created, request)
			if created.Code != http.StatusCreated {
				t.Fatalf("create response = %d, %s", created.Code, created.Body.String())
			}
			list := httptest.NewRecorder()
			server.Handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/tasks", nil))
			if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"name":"Task"`) {
				t.Errorf("list response = %d, %s", list.Code, list.Body.String())
			}
		})
	}
}

func TestRunInvalidAddress(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), "invalid address", testLogger())
	var addressErr *net.AddrError
	if !errors.As(err, &addressErr) {
		t.Errorf("error = %v, want a wrapped address error", err)
	}
}

func TestServeListenerFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("listener failed")
	listener := newTestListener()
	listener.acceptErr = want
	err := serve(context.Background(), &http.Server{}, listener, testLogger())
	if !errors.Is(err, want) {
		t.Errorf("error = %v, want wrapped %v", err, want)
	}
	waitFor(t, listener.closed, "listener close")
}

func TestServeCanceledBeforeStart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	listener := newTestListener()
	if err := serve(ctx, &http.Server{}, listener, testLogger()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, listener.closed, "listener close")
}

func TestServeExternalClose(t *testing.T) {
	t.Parallel()
	server := &http.Server{}
	t.Cleanup(func() { _ = server.Close() })
	listener := newTestListener()
	result := make(chan error, 1)
	go func() {
		result <- serve(context.Background(), server, listener, testLogger())
	}()
	waitFor(t, listener.accepting, "server start")
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	waitForResult(t, result)
}

func TestServeWaitsForActiveRequest(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	finishRequest := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finishRequest)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		if r.Context().Err() != nil {
			t.Error("graceful shutdown canceled an active request")
		}
		w.WriteHeader(http.StatusNoContent)
	})}
	t.Cleanup(func() { _ = server.Close() })
	listener := newTestListener()
	client, connection := net.Pipe()
	defer client.Close()
	t.Cleanup(func() { _ = connection.Close() })
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	listener.connections <- connection
	result := make(chan error, 1)
	go func() {
		result <- serve(ctx, server, listener, testLogger())
	}()
	request, err := http.NewRequest(http.MethodGet, "http://localhost/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Close = true
	if err := request.Write(client); err != nil {
		t.Fatal(err)
	}
	waitFor(t, started, "active request")
	cancel()
	waitFor(t, listener.closed, "shutdown start")
	select {
	case err := <-result:
		t.Fatalf("server exited before the active request completed: %v", err)
	default:
	}
	finishRequest()
	response, err := http.ReadResponse(bufio.NewReader(client), request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("response status = %d, want 204", response.StatusCode)
	}
	waitForResult(t, result)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitFor(t *testing.T, done <-chan struct{}, action string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", action)
	}
}

func waitForResult(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

type testListener struct {
	connections chan net.Conn
	closed      chan struct{}
	accepting   chan struct{}
	acceptOnce  sync.Once
	closeOnce   sync.Once
	acceptErr   error
}

func newTestListener() *testListener {
	return &testListener{
		connections: make(chan net.Conn, 1),
		closed:      make(chan struct{}),
		accepting:   make(chan struct{}),
	}
}

func (l *testListener) Accept() (net.Conn, error) {
	l.acceptOnce.Do(func() { close(l.accepting) })
	if l.acceptErr != nil {
		return nil, l.acceptErr
	}
	select {
	case connection := <-l.connections:
		return connection, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *testListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *testListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}
