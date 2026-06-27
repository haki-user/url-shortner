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
	"tinyurl/internal/health"
	"tinyurl/internal/link/adapters/codegen"
	"tinyurl/internal/link/adapters/httpapi"
	linkmemory "tinyurl/internal/link/adapters/memory"
	linkpostgres "tinyurl/internal/link/adapters/postgres"
	"tinyurl/internal/link/adapters/system"
	"tinyurl/internal/link/application"
	linkports "tinyurl/internal/link/ports"
	storagepostgres "tinyurl/internal/storage/postgres"
)

type storageDependencies struct {
	repository        linkports.LinkRepository
	idempotencyStore  linkports.IdempotencyStore
	analyticsRecorder analyticsports.RedirectEventRecorder
	readinessChecker  health.Checker
	cleanup           func()
}

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

	storage, err := buildStorage(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build storage: %w", err)
	}
	defer storage.cleanup()

	generator := codegen.NewBase62Generator()
	clock := system.SystemClock{}

	createGeneratedLink := application.NewCreateGeneratedLink(
		storage.repository,
		generator,
		clock,
		storage.idempotencyStore,
	)
	redirectLink := application.NewRedirectLink(storage.repository, clock)
	getManagedLink := application.NewGetManagedLink(storage.repository)
	changeLinkStatus := application.NewChangeLinkStatus(storage.repository, clock)

	linkHandler := httpapi.NewHandler(
		createGeneratedLink,
		redirectLink,
		cfg.BaseURL,
		httpapi.WithAnalytics(storage.analyticsRecorder, clock),
	)
	managementHandler := httpapi.NewManagementHandler(
		getManagedLink,
		changeLinkStatus,
	)

	healthHandler := health.NewHandler(storage.readinessChecker)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler.Liveness)
	mux.HandleFunc("GET /readyz", healthHandler.Readiness)
	mux.HandleFunc("GET /v1/links/{code}", managementHandler.Get)
	mux.HandleFunc("PATCH /v1/links/{code}", managementHandler.Patch)
	mux.Handle("/", linkHandler)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
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

func buildStorage(
	ctx context.Context,
	cfg config.Config,
) (storageDependencies, error) {
	switch cfg.Storage {
	case config.StorageMemory:
		log.Println("using memory storage")

		return storageDependencies{
			repository:        linkmemory.NewRepository(),
			idempotencyStore:  linkmemory.NewIdempotencyStore(),
			analyticsRecorder: analyticsmemory.NewRedirectEventRecorder(),
			readinessChecker: health.CheckerFunc(
				func(context.Context) error {
					return nil
				},
			),
			cleanup: func() {},
		}, nil

	case config.StoragePostgres:
		log.Println("using postgres storage")

		pool, err := storagepostgres.OpenPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return storageDependencies{}, fmt.Errorf(
				"open postgres pool: %w",
				err,
			)
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return storageDependencies{}, fmt.Errorf(
				"ping postgres: %w",
				err,
			)
		}

		return storageDependencies{
			repository:        linkpostgres.NewRepository(pool),
			idempotencyStore:  linkpostgres.NewIdempotencyStore(pool),
			analyticsRecorder: analyticspostgres.NewRedirectEventRecorder(pool),
			readinessChecker:  health.CheckerFunc(pool.Ping),
			cleanup:           pool.Close,
		}, nil

	default:
		return storageDependencies{}, fmt.Errorf(
			"unsupported storage mode: %q",
			cfg.Storage,
		)
	}
}
