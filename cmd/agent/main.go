package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hyfleet/hyfleet/internal/agent"
	"github.com/hyfleet/hyfleet/internal/buildinfo"
	"github.com/hyfleet/hyfleet/internal/config"
)

func main() {
	configPath := flag.String("config", "/etc/hyfleet/agent.yaml", "path to Agent YAML config")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("hyfleet-agent %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.LoadAgent(*configPath)
	if err != nil {
		logger.Error("load configuration failed", "error", err)
		os.Exit(1)
	}
	if *checkConfig {
		fmt.Println("Agent configuration is valid")
		return
	}
	runner, err := agent.New(cfg, logger)
	if err != nil {
		logger.Error("initialize Agent failed", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runner.Run(ctx); err != nil {
		logger.Error("Agent stopped", "error", err)
		os.Exit(1)
	}
}
