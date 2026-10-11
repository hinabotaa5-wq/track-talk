package main

import (
	"log"
	"os"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := migrate.Up(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString("migrate up: ok\n")
}
