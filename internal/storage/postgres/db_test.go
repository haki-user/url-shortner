package postgres

import (
	"context"
	"testing"
)

func TestOpenPoolReturnsErrorForInvalidDatabaseURL(t *testing.T) {
	pool, err := OpenPool(context.Background(), "://bad-url")
	if err == nil {
		if pool != nil {
			pool.Close()
		}

		t.Fatal("expected error, got nil")
	}

	if pool != nil {
		t.Fatalf("expected nil pool, got %v", pool)
	}
}

func TestOpenPoolCreatesPoolForValidDatabaseURL(t *testing.T) {
	pool, err := OpenPool(
		context.Background(),
		"postgres://tinyurl:tinyurl@localhost:5433/tinyurl?sslmode=disable",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected pool, got nil")
	}
}
