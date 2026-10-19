// Command server runs the Flashbacks registry service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flashbacks/registry/internal/config"
	"github.com/flashbacks/registry/internal/httpapi"
	"github.com/flashbacks/registry/internal/registry"
	"github.com/flashbacks/registry/internal/secrets"
	"github.com/flashbacks/registry/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load registry config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	embedded, err := registry.Start(ctx, *cfg)
	if err != nil {
		logger.Error("start embedded etcd", "error", err)
		os.Exit(1)
	}
	defer func() { _ = embedded.Close() }()

	codec, err := secrets.NewFromBase64(cfg.MasterKey)
	if err != nil {
		logger.Error("create secrets codec", "error", err)
		os.Exit(1)
	}

	api := httpapi.New(store.NewEtcdStore(embedded.Client()), codec, cfg.AuthToken)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("registry facade listening", "addr", cfg.HTTPAddr, "etcd", embedded.URL())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("facade server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("facade shutdown", "error", err)
	}
}
