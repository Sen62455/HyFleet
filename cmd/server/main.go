package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hyfleet/hyfleet/internal/buildinfo"
	"github.com/hyfleet/hyfleet/internal/config"
	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/server"
	"github.com/hyfleet/hyfleet/internal/store"
)

func main() {
	configPath := flag.String("config", "", "path to server YAML config")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit")
	backupDatabase := flag.String("backup-database", "", "write a consistent database backup and exit")
	checkDatabase := flag.String("check-database", "", "validate a database backup and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("hyfleet-server %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *checkDatabase != "" {
		if *backupDatabase != "" || *checkConfig {
			logger.Error("database check cannot be combined with another one-shot action")
			os.Exit(2)
		}
		if err := store.CheckDatabase(context.Background(), *checkDatabase); err != nil {
			logger.Error("database validation failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("database backup is valid")
		return
	}
	cfg, err := config.LoadServer(*configPath)
	if err != nil {
		logger.Error("load configuration failed", "error", err)
		os.Exit(1)
	}
	if *checkConfig {
		if *backupDatabase != "" {
			logger.Error("config check cannot be combined with database backup")
			os.Exit(2)
		}
		fmt.Println("server configuration is valid")
		return
	}
	if *backupDatabase != "" {
		if err := store.BackupDatabase(context.Background(), cfg.DatabasePath, *backupDatabase); err != nil {
			logger.Error("database backup failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("database backup created")
		return
	}
	masterKey, created, err := cryptoutil.LoadOrCreateKey(cfg.MasterKeyFile)
	if err != nil {
		logger.Error("load master key failed", "error", err)
		os.Exit(1)
	}
	if created {
		logger.Warn("created new master key; back it up separately", "path", cfg.MasterKeyFile)
	}
	ctx := context.Background()
	database, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	application, err := server.New(cfg, database, masterKey, logger)
	if err != nil {
		logger.Error("initialize server failed", "error", err)
		os.Exit(1)
	}
	handler, err := application.Handler()
	if err != nil {
		logger.Error("initialize HTTP handler failed", "error", err)
		os.Exit(1)
	}
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 * 1024,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runMaintenance(shutdownContext, database, logger, cfg.OfflineAfter)
	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("HTTP shutdown failed", "error", err)
		}
	}()
	logger.Info("HyFleet server listening", "address", cfg.Listen, "version", buildinfo.Version)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}

func runMaintenance(
	ctx context.Context,
	database *store.Store,
	logger *slog.Logger,
	offlineAfter time.Duration,
) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		now := time.Now().UTC()
		if count, err := database.EnforceExpiredUsers(ctx, now, 100); err != nil {
			if ctx.Err() == nil {
				logger.Error("enforce expired users failed", "error", err)
			}
		} else if count > 0 {
			logger.Info("expired users enforced", "count", count)
		}
		if err := database.ReconcileAlerts(ctx, now, offlineAfter, 5*time.Minute); err != nil && ctx.Err() == nil {
			logger.Error("reconcile alerts failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
