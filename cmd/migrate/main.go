// Command migrate applies or inspects the append-only database migrations.
package main

import (
	"fmt"
	"os"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
}

func run() error {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if len(os.Args) > 2 || (command != "up" && command != "status") {
		return fmt.Errorf("usage: migrate [up|status]")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	opts := database.OptionsFromConfig(cfg)
	opts.AutoMigrate, opts.Seed, opts.RequireSchema = false, false, false
	db, err := database.Open(opts)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	if command == "up" {
		if err := database.Migrate(db); err != nil {
			return err
		}
	}
	status, err := database.Status(db)
	if err != nil {
		return err
	}
	for _, item := range status {
		fmt.Printf("%s\tapplied=%t\n", item.Version, item.Applied)
	}
	return nil
}
