package ports

import "context"

type CodeGenerator interface {
	Generate(ctx context.Context) (string, error)
}
