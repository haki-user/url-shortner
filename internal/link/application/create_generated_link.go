package application

import (
	"context"
	"errors"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type CreateGeneratedLinkRequest struct {
	Destination    string
	OwnerID        string
	ExpiresAt      *time.Time
	IdempotencyKey string
}

type CreateGeneratedLink struct {
	repository       ports.LinkRepository
	generator        ports.CodeGenerator
	clock            ports.Clock
	idempotencyStore ports.IdempotencyStore
}

func NewCreateGeneratedLink(
	repository ports.LinkRepository,
	generator ports.CodeGenerator,
	clock ports.Clock,
	idempotencyStores ...ports.IdempotencyStore,
) CreateGeneratedLink {
	var idempotencyStore ports.IdempotencyStore
	if len(idempotencyStores) > 0 {
		idempotencyStore = idempotencyStores[0]
	}
	return CreateGeneratedLink{
		repository:       repository,
		generator:        generator,
		clock:            clock,
		idempotencyStore: idempotencyStore,
	}
}

func (c CreateGeneratedLink) Execute(
	ctx context.Context,
	request CreateGeneratedLinkRequest,
) (domain.Link, error) {
	if request.IdempotencyKey != "" && c.idempotencyStore != nil {
		link, err := c.idempotencyStore.Get(ctx, request.OwnerID, request.IdempotencyKey)
		if err == nil {
			return link, err
		}

		if !errors.Is(err, ports.ErrIdempotencyKeyNotFound) {
			return domain.Link{}, err
		}
	}

	destination, err := domain.NewDestinationURL(request.Destination)
	if err != nil {
		return domain.Link{}, err
	}

	code, err := c.generator.Generate(ctx)
	if err != nil {
		return domain.Link{}, err
	}

	now := c.clock.Now()

	link, err := domain.NewLink(code, destination, request.OwnerID, now, request.ExpiresAt)
	if err != nil {
		return domain.Link{}, err
	}

	if err := c.repository.Insert(ctx, link); err != nil {
		return domain.Link{}, err
	}

	if request.IdempotencyKey != "" && c.idempotencyStore != nil {
		if err := c.idempotencyStore.Save(ctx, request.OwnerID, request.IdempotencyKey, link); err != nil {
			return domain.Link{}, err
		}
	}

	return link, nil
}
