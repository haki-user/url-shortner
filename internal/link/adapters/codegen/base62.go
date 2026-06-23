package codegen

import (
	"context"
	"sync"

	"tinyurl/internal/link/ports"
)

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var _ ports.CodeGenerator = (*Base62Generator)(nil)

type Base62Generator struct {
	mu      sync.Mutex
	counter uint64
}

func NewBase62Generator() *Base62Generator {
	return &Base62Generator{}
}

func (g *Base62Generator) Generate(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.counter++

	return encodeBase62(g.counter), nil
}

func encodeBase62(n uint64) string {
	if n == 0 {
		return "0"
	}

	const base = uint64(len(base62Alphabet))

	var encoded []byte
	for n > 0 {
		remainder := n % base
		encoded = append(encoded, base62Alphabet[remainder])
		n = n / base
	}

	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}

	return string(encoded)
}
