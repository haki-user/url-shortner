package application

import (
	"context"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

// RepositoryResolver resolves redirect mappings from the source-of-truth repository.
type RepositoryResolver struct {
	repository ports.LinkRepository
}

// NewRepositoryResolver adapts a LinkRepository into a LinkResolver.
// The repository remains the source of truth when no cache is configured.
func NewRepositoryResolver(repository ports.LinkRepository) RepositoryResolver {
	return RepositoryResolver{
		repository: repository,
	}
}

func (r RepositoryResolver) Resolve(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	link, err := r.repository.FindByCode(ctx, code)
	if err != nil {
		return domain.RedirectMapping{}, err
	}

	return domain.RedirectMappingFromLink(link)
}
