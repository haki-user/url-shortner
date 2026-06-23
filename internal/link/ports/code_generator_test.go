package ports

import (
	"context"
	"errors"
	"testing"
)

var _ CodeGenerator = (*fakeCodeGenerator)(nil)

type fakeCodeGenerator struct {
	code string
	err  error
}

func (f fakeCodeGenerator) Generate(ctx context.Context) (string, error) {
	return f.code, f.err
}

func TestFakeCodeGeneratorImplementsGenerator(t *testing.T) {
	expectedCode := "abc123"
	expectedErr := errors.New("generate failed")

	generator := fakeCodeGenerator{
		code: expectedCode,
		err:  expectedErr,
	}

	actualCode, actualErr := generator.Generate(context.Background())

	if actualCode != expectedCode {
		t.Fatalf("expected code %v, got %v", expectedCode, actualCode)
	}

	if !errors.Is(actualErr, expectedErr) {
		t.Fatalf("expected err %v, got %v", expectedErr, actualErr)
	}
}
