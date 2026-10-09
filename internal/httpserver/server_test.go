package httpserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeCancelsAndDrainsRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
		<-release
	})}
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, server, listener) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("request not canceled")
	}
	select {
	case err := <-done:
		t.Fatalf("returned before handler drained: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
	<-clientDone
	rebound, err := net.Listen("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	rebound.Close()
}

func TestRunReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := Run(context.Background(), &http.Server{Addr: listener.Addr().String()}); err == nil {
		t.Fatal("expected bind failure")
	}
}
