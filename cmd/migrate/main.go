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
	direction := flag.String("direction", "up", "migration direction: up|down|version")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	switch *direction {
	case "up":
		if err := migrate.Up(cfg.DB.DSN()); err != nil {
			slog.Error("migrate up", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	case "down":
		if err := migrate.Down(cfg.DB.DSN()); err != nil {
			slog.Error("migrate down", "err", err)
			os.Exit(1)
		}
		slog.Info("last migration rolled back")
	case "version":
		version, dirty, err := migrate.Version(cfg.DB.DSN())
		if err != nil {
			slog.Error("migrate version", "err", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
	default:
		slog.Error("unknown migration direction", "direction", *direction)
		os.Exit(1)
	}

	os.Exit(0)
}
