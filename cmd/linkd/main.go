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
	linkredis "tinyurl/internal/link/adapters/redis"
	"tinyurl/internal/link/adapters/system"
	"tinyurl/internal/link/application"
	linkports "tinyurl/internal/link/ports"
	"tinyurl/internal/metrics"
	storagepostgres "tinyurl/internal/storage/postgres"

	redisclient "github.com/redis/go-redis/v9"
)

type storageDependencies struct {
	repository        linkports.LinkRepository
	idempotencyStore  linkports.IdempotencyStore
	analyticsRecorder analyticsports.RedirectEventRecorder
	readinessChecker  health.Checker
	diagnostics       []health.ComponentCheck
	cleanup           func()
}

type resolverDependencies struct {
	resolver    linkports.LinkResolver
	repository  linkports.LinkRepository
	diagnostics []health.ComponentCheck
	cleanup     func()
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
	metricsRecorder := metrics.NewRecorder()

	// main is the composition root: it constructs concrete infrastructure
	// dependencies and injects them into application components through interfaces.
	resolverDependencies, err := buildLinkResolver(
		cfg,
		storage.repository,
		clock,
		metricsRecorder,
	)
	if err != nil {
		return fmt.Errorf("build link resolver: %w", err)
	}
	defer resolverDependencies.cleanup()

	repository := resolverDependencies.repository

	createGeneratedLink := application.NewCreateGeneratedLink(
		repository,
		generator,
		clock,
		storage.idempotencyStore,
	)
	redirectLink := application.NewRedirectLink(
		resolverDependencies.resolver,
		clock,
	)
	getManagedLink := application.NewGetManagedLink(repository)
	changeLinkStatus := application.NewChangeLinkStatus(repository, clock)
	changeLinkDestination := application.NewChangeLinkDestination(
		repository,
		clock,
	)
	changeLinkExpiration := application.NewChangeLinkExpiration(
		repository,
		clock,
	)

	linkHandler := httpapi.NewHandler(
		createGeneratedLink,
		redirectLink,
		cfg.BaseURL,
		httpapi.WithAnalytics(storage.analyticsRecorder, clock),
		httpapi.WithMetrics(metricsRecorder),
	)
	managementHandler := httpapi.NewManagementHandler(
		getManagedLink,
		changeLinkStatus,
		changeLinkDestination,
		changeLinkExpiration,
	)

	diagnostics := append(
		[]health.ComponentCheck{},
		storage.diagnostics...,
	)
	diagnostics = append(diagnostics, resolverDependencies.diagnostics...)
	healthHandler := health.NewHandler(
		storage.readinessChecker,
		diagnostics,
		cfg.DiagnosticsToken,
	)
	metricsHandler := metrics.NewHandler(metricsRecorder, cfg.DiagnosticsToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler.Liveness)
	mux.HandleFunc("GET /readyz", healthHandler.Readiness)
	mux.HandleFunc("GET /internal/diagnostics", healthHandler.Diagnostics)
	mux.Handle("GET /internal/metrics", metricsHandler)
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

		memoryChecker := health.CheckerFunc(
			func(context.Context) error {
				return nil
			},
		)

		return storageDependencies{
			repository:        linkmemory.NewRepository(),
			idempotencyStore:  linkmemory.NewIdempotencyStore(),
			analyticsRecorder: analyticsmemory.NewRedirectEventRecorder(),
			readinessChecker:  memoryChecker,
			diagnostics: []health.ComponentCheck{
				{
					Name:     "memory",
					Required: true,
					Checker:  memoryChecker,
				},
			},
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

		postgresChecker := health.CheckerFunc(pool.Ping)

		return storageDependencies{
			repository:        linkpostgres.NewRepository(pool),
			idempotencyStore:  linkpostgres.NewIdempotencyStore(pool),
			analyticsRecorder: analyticspostgres.NewRedirectEventRecorder(pool),
			readinessChecker:  postgresChecker,
			diagnostics: []health.ComponentCheck{
				{
					Name:     "postgres",
					Required: true,
					Checker:  postgresChecker,
				},
			},
			cleanup: pool.Close,
		}, nil

	default:
		return storageDependencies{}, fmt.Errorf(
			"unsupported storage mode: %q",
			cfg.Storage,
		)
	}
}

func buildLinkResolver(
	cfg config.Config,
	repository linkports.LinkRepository,
	clock linkports.Clock,
	metricsRecorder application.RedirectCacheMetrics,
) (resolverDependencies, error) {
	source := application.NewRepositoryResolver(repository)

	switch cfg.Cache {
	case config.CacheNone:
		log.Println("redirect cache disabled")

		return resolverDependencies{
			resolver:    source,
			repository:  repository,
			diagnostics: nil,
			cleanup:     func() {},
		}, nil

	case config.CacheRedis:
		options, err := redisclient.ParseURL(cfg.RedisURL)
		if err != nil {
			return resolverDependencies{}, fmt.Errorf(
				"parse Redis URL: %w", err,
			)
		}

		// Make per-operation context deadlines bound Redis network I/O.
		options.ContextTimeoutEnabled = true

		client := redisclient.NewClient(options)
		cache := linkredis.NewRedirectCache(client)

		cacheConfig := application.RedirectCacheConfig{
			OperationTimeout: cfg.CacheOperationTimeout,
			ActiveTTL:        cfg.CacheActiveTTL,
			InactiveTTL:      cfg.CacheInactiveTTL,
		}

		resolver, err := application.NewCacheAsideResolver(
			cache,
			source,
			clock,
			cacheConfig,
			application.WithRedirectCacheMetrics(metricsRecorder),
		)
		if err != nil {
			_ = client.Close()
			return resolverDependencies{}, fmt.Errorf(
				"create cache aside resolver: %w",
				err,
			)
		}

		refresher, err := application.NewRedirectCacheRefresher(
			cache,
			clock,
			cacheConfig,
		)
		if err != nil {
			_ = client.Close()
			return resolverDependencies{}, fmt.Errorf(
				"create redirect cache refresher: %w",
				err,
			)
		}

		refreshingRepository := application.NewCacheRefreshingRepository(
			repository,
			refresher,
		)

		log.Println("using Redis redirect cache")

		return resolverDependencies{
			resolver:   resolver,
			repository: refreshingRepository,
			diagnostics: []health.ComponentCheck{
				{
					Name:     "redis",
					Required: false,
					Checker: health.CheckerFunc(
						func(ctx context.Context) error {
							return client.Ping(ctx).Err()
						},
					),
				},
			},
			cleanup: func() {
				_ = client.Close()
			},
		}, nil

	default:
		return resolverDependencies{}, fmt.Errorf(
			"unsupported cache mode: %q",
			cfg.Cache,
		)
	}
}
