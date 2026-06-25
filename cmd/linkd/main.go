package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	analyticsmemory "tinyurl/internal/analytics/adapters/memory"
	analyticspostgres "tinyurl/internal/analytics/adapters/postgres"
	analyticsports "tinyurl/internal/analytics/ports"
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
	const addr = ":8080"
	const baseURL = "http://localhost:8080"

	ctx := context.Background()

	repository, idempotencyStore, analyticsRecorder, cleanup := buildStorage(ctx)

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
		baseURL,
		httpapi.WithAnalytics(analyticsRecorder, clock),
	)

	log.Printf("linkd listening on %s", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		cleanup()
		log.Fatalf("listen and serve: %v", err)
	}
}

func buildStorage(ctx context.Context) (
	linkports.LinkRepository,
	linkports.IdempotencyStore,
	analyticsports.RedirectEventRecorder,
	func(),
) {
	storage := strings.ToLower(strings.TrimSpace(os.Getenv("TINYURL_STORAGE")))
	if storage == "" {
		storage = "memory"
	}

	switch storage {
	case "memory":
		log.Println("using memory storage")

		return linkmemory.NewRepository(),
			linkmemory.NewIdempotencyStore(),
			analyticsmemory.NewRedirectEventRecorder(),
			func() {}

	case "postgres":
		log.Println("using postgres storage")

		databaseURL := strings.TrimSpace(os.Getenv("TINYURL_DATABASE_URL"))
		if databaseURL == "" {
			log.Fatal("TINYURL_DATABASE_URL is required when TINYURL_STORAGE=postgres")
		}

		pool, err := storagepostgres.OpenPool(ctx, databaseURL)
		if err != nil {
			log.Fatalf("open postgres pool: %v", err)
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			log.Fatalf("ping postgres: %v", err)
		}

		return linkpostgres.NewRepository(pool),
			linkpostgres.NewIdempotencyStore(pool),
			analyticspostgres.NewRedirectEventRecorder(pool),
			pool.Close

	default:
		log.Fatalf("unsupported TINYURL_STORAGE %q, expected memory or postgres", storage)
		return nil, nil, nil, func() {}
	}
}
