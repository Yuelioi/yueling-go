// Package httpserver owns listener cancellation and request draining.
package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Run returns only after serving stops and active HTTP requests drain. Upgraded
// connections must be owned and closed by the protocol handler separately.
func Run(ctx context.Context, server *http.Server) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	return Serve(ctx, server, listener)
}

func Serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	server.BaseContext = func(net.Listener) context.Context { return ctx }
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := server.Shutdown(shutdown)
	if err != nil {
		_ = server.Close()
	}
	serveErr := <-done
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(err, serveErr)
}
