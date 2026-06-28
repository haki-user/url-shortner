package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const migrationAdvisoryLockID int64 = 824662419235

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]+)_.+\.sql$`)

type migration struct {
	version  int64
	name     string
	sql      string
	checksum string
}

func RunMigrations(
	ctx context.Context,
	databaseURL string,
	directory string,
) ([]string, error) {
	migrations, err := discoverMigrations(directory)
	if err != nil {
		return nil, err
	}

	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	connection, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to Postgres: %w", err)
	}
	defer connection.Close(context.Background())

	if _, err := connection.Exec(
		ctx,
		"select pg_advisory_lock($1)",
		migrationAdvisoryLockID,
	); err != nil {
		return nil, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer releaseMigrationLock(connection)

	if _, err := connection.Exec(ctx, `
		create table if not exists schema_migrations (
			version bigint primary key,
			name text not null,
			checksum text not null,
			applied_at timestamptz not null default now()
		)
	`); err != nil {
		return nil, fmt.Errorf("create migration history: %w", err)
	}

	applied := make([]string, 0, len(migrations))
	for _, candidate := range migrations {
		wasApplied, err := migrationWasApplied(ctx, connection, candidate)
		if err != nil {
			return nil, err
		}
		if wasApplied {
			continue
		}

		if err := applyMigration(ctx, connection, candidate); err != nil {
			return nil, err
		}
		applied = append(applied, candidate.name)
	}

	return applied, nil
}

func discoverMigrations(directory string) ([]migration, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %q: %w", directory, err)
	}

	migrations := make([]migration, 0, len(entries))
	versions := make(map[int64]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := migrationFilenamePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}

		if previous, exists := versions[version]; exists {
			return nil, fmt.Errorf(
				"duplicate migration version %d in %q and %q",
				version,
				previous,
				entry.Name(),
			)
		}
		versions[version] = entry.Name()

		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		digest := sha256.Sum256(contents)
		migrations = append(migrations, migration{
			version:  version,
			name:     entry.Name(),
			sql:      string(contents),
			checksum: hex.EncodeToString(digest[:]),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func migrationWasApplied(
	ctx context.Context,
	connection *pgx.Conn,
	candidate migration,
) (bool, error) {
	var storedChecksum string
	err := connection.QueryRow(
		ctx,
		"select checksum from schema_migrations where version = $1",
		candidate.version,
	).Scan(&storedChecksum)

	switch {
	case err == nil:
		if storedChecksum != candidate.checksum {
			return false, fmt.Errorf(
				"migration %q checksum changed after it was applied",
				candidate.name,
			)
		}
		return true, nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf(
			"read migration history for %q: %w",
			candidate.name,
			err,
		)
	}
}

func applyMigration(
	ctx context.Context,
	connection *pgx.Conn,
	candidate migration,
) error {
	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", candidate.name, err)
	}
	defer transaction.Rollback(context.Background())

	if _, err := transaction.Exec(ctx, candidate.sql); err != nil {
		return fmt.Errorf("execute migration %q: %w", candidate.name, err)
	}

	if _, err := transaction.Exec(
		ctx,
		`insert into schema_migrations (version, name, checksum)
		 values ($1, $2, $3)`,
		candidate.version,
		candidate.name,
		candidate.checksum,
	); err != nil {
		return fmt.Errorf("record migration %q: %w", candidate.name, err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %q: %w", candidate.name, err)
	}

	return nil
}

func releaseMigrationLock(connection *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = connection.Exec(
		ctx,
		"select pg_advisory_unlock($1)",
		migrationAdvisoryLockID,
	)
}
