package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sherrytsaii/gogohomework/internal/httpapi"
	"github.com/sherrytsaii/gogohomework/internal/storage/memory"
	"github.com/sherrytsaii/gogohomework/internal/task"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv("HTTP_ADDR"), logger); err != nil {
		logger.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
}

func newServer(address string, logger *slog.Logger) *http.Server {
	if address == "" {
		address = ":8080"
	}
	repository := memory.NewTaskRepository()
	service := task.NewService(repository)
	return &http.Server{
		Addr:              address,
		Handler:           httpapi.NewRouter(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

func run(ctx context.Context, address string, logger *slog.Logger) error {
	server := newServer(address, logger)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	return serve(ctx, server, listener, logger)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *slog.Logger) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	logger.Info("HTTP server listening", "address", listener.Addr().String())

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("HTTP server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			closeErr := server.Close()
			<-serveErrors
			return fmt.Errorf("shutdown HTTP: %w", errors.Join(err, closeErr))
		}
		if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		logger.Info("HTTP server stopped")
		return nil
	}
}
