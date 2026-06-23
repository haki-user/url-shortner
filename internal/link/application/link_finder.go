package application

import (
	"context"
	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type LinkFinder struct {
	repository ports.LinkRepository
}

func (f LinkFinder) Find(ctx context.Context, code string) (domain.Link, error) {
	return f.repository.FindByCode(ctx, code)
}

func NewLinkFinder(repository ports.LinkRepository) LinkFinder {
	return LinkFinder{repository: repository}
}
