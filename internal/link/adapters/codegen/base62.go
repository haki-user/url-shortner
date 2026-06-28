package codegen

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	"tinyurl/internal/link/ports"
)

const (
	base62Alphabet    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	defaultCodeLength = 8
	unbiasedByteLimit = 256 - (256 % len(base62Alphabet))
)

var _ ports.CodeGenerator = (*Base62Generator)(nil)

// Base62Generator creates independently generated, URL-safe random codes.
type Base62Generator struct {
	reader io.Reader
	length int
}

func NewBase62Generator() *Base62Generator {
	return &Base62Generator{
		reader: rand.Reader,
		length: defaultCodeLength,
	}
}

func (g *Base62Generator) Generate(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	code := make([]byte, 0, g.length)
	randomBytes := make([]byte, g.length)

	for len(code) < g.length {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		if _, err := io.ReadFull(g.reader, randomBytes); err != nil {
			return "", fmt.Errorf("read random bytes: %w", err)
		}

		for _, value := range randomBytes {
			if int(value) >= unbiasedByteLimit {
				continue
			}

			code = append(code, base62Alphabet[int(value)%len(base62Alphabet)])
			if len(code) == g.length {
				break
			}
		}
	}

	return string(code), nil
}
