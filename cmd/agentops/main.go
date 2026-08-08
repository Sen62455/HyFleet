package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hyfleet/hyfleet/internal/buildinfo"
	"github.com/hyfleet/hyfleet/internal/config"
	"github.com/hyfleet/hyfleet/internal/nodeops"
)

func main() {
	configPath := flag.String("config", "", "path to Agent YAML config")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("hyfleet-agent-ops %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.LoadAgent(*configPath)
	if err != nil {
		logger.Error("load configuration failed", "error", err)
		os.Exit(1)
	}
	helper, err := nodeops.NewHelper(cfg.ServiceUnit, cfg.CoreConfigPath)
	if err != nil {
		logger.Error("initialize operations helper failed", "error", err)
		os.Exit(1)
	}
	if *checkConfig {
		fmt.Println("operations helper configuration is valid")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := helper.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		logger.Error("operations helper request failed", "error", err)
		os.Exit(1)
	}
}
