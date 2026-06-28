package codegen

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestBase62GeneratorGenerateReturnsFixedLengthURLSafeCode(t *testing.T) {
	generator := NewBase62Generator()

	code, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(code) != defaultCodeLength {
		t.Fatalf("expected code length %d, got %d", defaultCodeLength, len(code))
	}

	for _, character := range code {
		if !strings.ContainsRune(base62Alphabet, character) {
			t.Fatalf("generated non-Base62 character %q", character)
		}
	}
}

func TestBase62GeneratorUsesRejectionSampling(t *testing.T) {
	generator := &Base62Generator{
		reader: bytes.NewReader([]byte{0, 61, 248, 62, 123, 0, 0, 0}),
		length: 4,
	}

	code, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if code != "0Z0Z" {
		t.Fatalf("expected deterministic code %q, got %q", "0Z0Z", code)
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

func TestBase62GeneratorRandomSourceFailureIsReturned(t *testing.T) {
	expectedErr := errors.New("random source failed")
	generator := &Base62Generator{
		reader: errorReader{err: expectedErr},
		length: defaultCodeLength,
	}

	code, err := generator.Generate(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if code != "" {
		t.Fatalf("expected empty code, got %q", code)
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
		t.Fatalf("expected no error, got %v", err)
	}

	seen := make(map[string]struct{}, totalCodes)
	for code := range codes {
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate code generated: %q", code)
		}

		seen[code] = struct{}{}
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
