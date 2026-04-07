package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"magaz/internal/config"
	"magaz/internal/db"
	"magaz/payment"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	// Microservice uses its own DB (or the same, separate table)
	database, err := db.New(cfg.PaymentDBURL)
	if err != nil {
		slog.Error("payment db connect failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run only the transactions migration
	if err := db.Migrate(database, "migrations"); err != nil {
		slog.Error("payment migrations failed", "err", err)
		os.Exit(1)
	}

	repo    := payment.NewRepository(database)
	svc     := payment.NewService(repo)
	h       := payment.NewHandler(svc, cfg.PaymentSecret)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.Logger)

	r.Post("/charge", h.Charge)
	r.Get("/status", h.Status)

	srv := &http.Server{
		Addr:         cfg.PaymentAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("payment service starting", "addr", cfg.PaymentAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("payment server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("payment service shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
