package main

import (
	"net/http"
	"os"
	"time"

	"log/slog"

	"github.com/ruth411/circle/internal/platform/config"
	"github.com/ruth411/circle/internal/platform/httpapi"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	addr := ":" + cfg.Port

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewServer(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("circle listening", "addr", addr, "env", cfg.AppEnv)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
