package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	storagepostgres "tinyurl/internal/storage/postgres"
)

const defaultMigrationsDirectory = "migrations/postgres"

func main() {
	if err := run(); err != nil {
		log.Printf("migration failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := strings.TrimSpace(os.Getenv("TINYURL_DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("TINYURL_DATABASE_URL is required")
	}

	migrationsDirectory := strings.TrimSpace(os.Getenv("TINYURL_MIGRATIONS_DIR"))
	if migrationsDirectory == "" {
		migrationsDirectory = defaultMigrationsDirectory
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	applied, err := storagepostgres.RunMigrations(
		ctx,
		databaseURL,
		migrationsDirectory,
	)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		log.Println("database schema is up to date")
		return nil
	}

	for _, name := range applied {
		log.Printf("applied migration %s", name)
	}

	return nil
}
