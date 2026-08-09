package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/migrate"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up|down|version")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	switch *direction {
	case "up":
		if err := migrate.Up(cfg.DB.DSN()); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("migrations applied")
	case "down":
		if err := migrate.Down(cfg.DB.DSN()); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		log.Println("last migration rolled back")
	case "version":
		version, dirty, err := migrate.Version(cfg.DB.DSN())
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
	default:
		log.Fatalf("unknown direction %q (use up|down|version)", *direction)
	}

	os.Exit(0)
}
