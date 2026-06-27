package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	analyticsmemory "tinyurl/internal/analytics/adapters/memory"
	analyticspostgres "tinyurl/internal/analytics/adapters/postgres"
	analyticsports "tinyurl/internal/analytics/ports"
	"tinyurl/internal/config"
	"tinyurl/internal/link/adapters/codegen"
	"tinyurl/internal/link/adapters/httpapi"
	linkmemory "tinyurl/internal/link/adapters/memory"
	linkpostgres "tinyurl/internal/link/adapters/postgres"
	"tinyurl/internal/link/adapters/system"
	"tinyurl/internal/link/application"
	linkports "tinyurl/internal/link/ports"
	storagepostgres "tinyurl/internal/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Printf("linkd stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repository, idempotencyStore, analyticsRecorder, cleanup, err := buildStorage(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build storage: %w", err)
	}
	defer cleanup()

	generator := codegen.NewBase62Generator()
	clock := system.SystemClock{}

	createGeneratedLink := application.NewCreateGeneratedLink(
		repository,
		generator,
		clock,
		idempotencyStore,
	)
	redirectLink := application.NewRedirectLink(repository, clock)

	handler := httpapi.NewHandler(
		createGeneratedLink,
		redirectLink,
		cfg.BaseURL,
		httpapi.WithAnalytics(analyticsRecorder, clock),
	)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}

		serverErrors <- nil
	}()

	log.Printf("linkd listening on %s", cfg.Addr)

	select {
	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf("listen and serve: %w", err)
		}

		return nil

	case <-ctx.Done():
		log.Println("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		if closeErr := server.Close(); closeErr != nil {
			return errors.Join(
				fmt.Errorf("shutdown server: %w", err),
				fmt.Errorf("force close server: %w", closeErr),
			)
		}

		return fmt.Errorf("shutdown server: %w", err)
	}

	if err := <-serverErrors; err != nil {
		return fmt.Errorf("server stopped during shutdown: %w", err)
	}

	log.Println("shutdown complete")

	return nil
}

func buildStorage(ctx context.Context, cfg config.Config) (
	linkports.LinkRepository,
	linkports.IdempotencyStore,
	analyticsports.RedirectEventRecorder,
	func(),
	error,
) {

	switch cfg.Storage {
	case config.StorageMemory:
		log.Println("using memory storage")

		return linkmemory.NewRepository(),
			linkmemory.NewIdempotencyStore(),
			analyticsmemory.NewRedirectEventRecorder(),
			func() {},
			nil

	case config.StoragePostgres:
		log.Println("using postgres storage")

		pool, err := storagepostgres.OpenPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("open postgres pool: %w", err)
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, nil, nil, fmt.Errorf("ping postgres: %w", err)
		}

		return linkpostgres.NewRepository(pool),
			linkpostgres.NewIdempotencyStore(pool),
			analyticspostgres.NewRedirectEventRecorder(pool),
			pool.Close,
			nil

	default:
		return nil, nil, nil, nil, fmt.Errorf("unsupported storage mode: %q", cfg.Storage)
	}
}
