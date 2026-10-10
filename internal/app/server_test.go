package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"testinbox/internal/config"
)

func TestPortConflict(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	err = Run(context.Background(), config.Config{HTTPAddr: l.Addr().String()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected listen error")
	}
}

func TestShutdown(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "drains", true: "deadline"}[timeout], func(t *testing.T) {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			handlerDone := make(chan struct{})
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				select {
				case <-release:
				case <-r.Context().Done():
				}
				w.WriteHeader(200)
				close(handlerDone)
			})
			grace := time.Second
			if timeout {
				grace = 30 * time.Millisecond
			}
			done := make(chan error, 1)
			go func() { done <- serve(ctx, l, handler, grace) }()
			client := &http.Client{Timeout: 2 * time.Second}
			defer client.CloseIdleConnections()
			result := make(chan error, 1)
			go func() {
				resp, err := client.Get("http://" + l.Addr().String())
				if err == nil {
					resp.Body.Close()
				}
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			if !timeout {
				select {
				case err := <-done:
					t.Fatalf("did not drain: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				release <- struct{}{}
			}
			select {
			case err := <-done:
				if timeout && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline, got %v", err)
				}
				if !timeout && err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("shutdown stuck")
			}
			select {
			case <-handlerDone:
			case <-time.After(time.Second):
				t.Fatal("handler leaked")
			}
			err = <-result
			if !timeout && err != nil {
				t.Fatal(err)
			}
			conn, err := net.DialTimeout("tcp", l.Addr().String(), time.Second)
			if err == nil {
				conn.Close()
				t.Fatal("listener remains open")
			}
		})
	}
}
