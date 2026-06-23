package codegen

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestBase62GeneratorGenerateReturnsFirstFiveCodes(t *testing.T) {
	generator := NewBase62Generator()

	tests := []string{"1", "2", "3", "4", "5"}

	for _, expected := range tests {
		actual, err := generator.Generate(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if actual != expected {
			t.Fatalf("expected code %q, got %q", expected, actual)
		}
	}
}

func TestEncodeBase62Boundaries(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		{name: "zero", input: 0, expected: "0"},
		{name: "one", input: 1, expected: "1"},
		{name: "nine", input: 9, expected: "9"},
		{name: "ten", input: 10, expected: "a"},
		{name: "thirty five", input: 35, expected: "z"},
		{name: "thirty six", input: 36, expected: "A"},
		{name: "sixty one", input: 61, expected: "Z"},
		{name: "sixty two", input: 62, expected: "10"},
		{name: "sixty three", input: 63, expected: "11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := encodeBase62(tt.input)
			if actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestBase62GeneratorCancelledContextReturnsContextCanceled(t *testing.T) {
	generator := NewBase62Generator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code, err := generator.Generate(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if code != "" {
		t.Fatalf("expected empty code, got %q", code)
	}
}

func TestBase62GeneratorCancelledGenerationDoesNotIncrementCounter(t *testing.T) {
	generator := NewBase62Generator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := generator.Generate(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	code, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != "1" {
		t.Fatalf("expected first successful code to be %q, got %q", "1", code)
	}
}

func TestBase62GeneratorConcurrentGenerationHasNoDuplicates(t *testing.T) {
	generator := NewBase62Generator()

	const totalCodes = 1000

	var wg sync.WaitGroup
	codes := make(chan string, totalCodes)
	errs := make(chan error, totalCodes)

	for i := 0; i < totalCodes; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			code, err := generator.Generate(context.Background())
			if err != nil {
				errs <- err
				return
			}

			codes <- code
		}()
	}

	wg.Wait()
	close(codes)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}

	seen := make(map[string]struct{}, totalCodes)
	for code := range codes {
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate code generated: %q", code)
		}

		seen[code] = struct{}{}
	}

	if len(seen) != totalCodes {
		t.Fatalf("expected %d unique codes, got %d", totalCodes, len(seen))
	}
}
