package main

import (
	"log"
	"net/http"

	analyticsmemory "tinyurl/internal/analytics/adapters/memory"
	"tinyurl/internal/link/adapters/codegen"
	"tinyurl/internal/link/adapters/httpapi"
	"tinyurl/internal/link/adapters/memory"
	"tinyurl/internal/link/adapters/system"
	"tinyurl/internal/link/application"
)

func main() {
	const addr = ":8080"
	const baseURL = "http://localhost:8080"

	repository := memory.NewRepository()
	idempotencyStore := memory.NewIdempotencyStore()
	analyticsRecorder := analyticsmemory.NewRedirectEventRecorder()
	generator := codegen.NewBase62Generator()
	clock := system.SystemClock{}

	createGeneratedLink := application.NewCreateGeneratedLink(repository, generator, clock, idempotencyStore)
	redirectLink := application.NewRedirectLink(repository, clock)

	handler := httpapi.NewHandler(
		createGeneratedLink,
		redirectLink,
		baseURL,
		httpapi.WithAnalytics(analyticsRecorder, clock),
	)

	log.Printf("linkd listening on %s", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
