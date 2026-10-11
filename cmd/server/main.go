// @title           Life Dashboard Hub API
// @version         1.0
// @description     Self-hosted Go backend for receiving Life Dashboard Companion payloads and reporting.
// @BasePath        /api/v1

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/awang-karisma/life-dashboard-hub/internal/config"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	initEmbedFrontend()

	srv := server.New(cfg, db)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		slog.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}
