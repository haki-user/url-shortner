package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverMigrationsOrdersByNumericVersion(t *testing.T) {
	directory := t.TempDir()
	writeMigrationFile(t, directory, "0010_tenth.sql", "select 10;")
	writeMigrationFile(t, directory, "0002_second.sql", "select 2;")
	writeMigrationFile(t, directory, "README.md", "ignored")

	migrations, err := discoverMigrations(directory)
	if err != nil {
		t.Fatalf("discover migrations: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	if migrations[0].version != 2 || migrations[0].name != "0002_second.sql" {
		t.Fatalf("unexpected first migration: %+v", migrations[0])
	}

	if migrations[1].version != 10 || migrations[1].name != "0010_tenth.sql" {
		t.Fatalf("unexpected second migration: %+v", migrations[1])
	}

	if len(migrations[0].checksum) != 64 {
		t.Fatalf("expected SHA-256 checksum, got %q", migrations[0].checksum)
	}
}

func TestDiscoverMigrationsRejectsDuplicateVersions(t *testing.T) {
	directory := t.TempDir()
	writeMigrationFile(t, directory, "0001_first.sql", "select 1;")
	writeMigrationFile(t, directory, "1_duplicate.sql", "select 2;")

	_, err := discoverMigrations(directory)
	if err == nil {
		t.Fatal("expected duplicate version error")
	}

	if !strings.Contains(err.Error(), "duplicate migration version 1") {
		t.Fatalf("expected duplicate version error, got %v", err)
	}
}

func TestDiscoverMigrationsReturnsMissingDirectoryError(t *testing.T) {
	_, err := discoverMigrations(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected missing directory error")
	}
}

func writeMigrationFile(
	t *testing.T,
	directory string,
	name string,
	contents string,
) {
	t.Helper()

	if err := os.WriteFile(
		filepath.Join(directory, name),
		[]byte(contents),
		0o600,
	); err != nil {
		t.Fatalf("write migration fixture: %v", err)
	}
}
