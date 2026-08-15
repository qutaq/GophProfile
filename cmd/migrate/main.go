package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/migrate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("component", "migrate")

	direction := flag.String("direction", "up", "migration direction: up|down|version")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	switch *direction {
	case "up":
		if err := migrate.Up(cfg.DB.DSN(), logger); err != nil {
			logger.Error("migrate up", "err", err)
			os.Exit(1)
		}
		logger.Info("migrations applied")
	case "down":
		if err := migrate.Down(cfg.DB.DSN(), logger); err != nil {
			logger.Error("migrate down", "err", err)
			os.Exit(1)
		}
		logger.Info("last migration rolled back")
	case "version":
		version, dirty, err := migrate.Version(cfg.DB.DSN(), logger)
		if err != nil {
			logger.Error("migrate version", "err", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
	default:
		logger.Error("unknown migration direction", "direction", *direction)
		os.Exit(1)
	}

	os.Exit(0)
}
