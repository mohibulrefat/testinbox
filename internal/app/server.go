package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"testinbox/internal/config"
	"testinbox/internal/httpapi"
)

const ShutdownTimeout = 10 * time.Second

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on HTTP address %s: %w", cfg.HTTPAddr, err)
	}
	host, _, _ := net.SplitHostPort(cfg.HTTPAddr)
	if !net.ParseIP(host).IsLoopback() {
		logger.Warn("HTTP is bound outside loopback; use only on trusted local networks", "address", cfg.HTTPAddr)
	}
	logger.Info("HTTP listening; storage is not initialized; SMTP is disabled", "address", listener.Addr().String())
	return serve(ctx, listener, httpapi.Handler(), ShutdownTimeout)
}

func serve(ctx context.Context, listener net.Listener, handler http.Handler, grace time.Duration) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-done
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		err := <-done
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}
