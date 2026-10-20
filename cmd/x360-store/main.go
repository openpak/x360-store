// Command x360-store serves OpenPak's Xbox 360 homebrew catalog and download queue (see internal/store).
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

	"github.com/openpak/x360-common/wire"
	"github.com/openpak/x360-store/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	addr := os.Getenv("X360_STORE_ADDR")
	if addr == "" {
		addr = ":10044"
	}
	catalog, err := store.LoadCatalog(os.Getenv("X360_STORE_CATALOG"))
	if err != nil {
		log.Error("catalog (X360_STORE_CATALOG)", "err", err)
		os.Exit(2)
	}
	h := &store.Handler{Catalog: catalog, Queues: &store.Queues{}, AdminKey: os.Getenv("X360_STORE_ADMIN_KEY"), Log: log}
	srv := &http.Server{Addr: addr, Handler: wire.Guard(os.Getenv("X360_GATEWAY_KEY"), h), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(sc)
	}()
	log.Info("x360-store listening", "addr", addr, "items", len(catalog.Items))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
