package application

import (
	"context"
	"errors"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

const maxCodeGenerationAttempts = 5

var ErrCodeGenerationExhausted = errors.New("code generation retries exhausted")

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

	var (
		link      domain.Link
		createdAt time.Time
	)
	for attempt := 0; attempt < maxCodeGenerationAttempts; attempt++ {
		code, err := c.generator.Generate(ctx)
		if err != nil {
			return domain.Link{}, err
		}

		if attempt == 0 {
			createdAt = c.clock.Now()
		}

		link, err = domain.NewLink(
			code,
			destination,
			request.OwnerID,
			createdAt,
			request.ExpiresAt,
		)
		if err != nil {
			return domain.Link{}, err
		}

		err = c.repository.Insert(ctx, link)
		if err == nil {
			break
		}

		if !errors.Is(err, ports.ErrLinkAlreadyExists) {
			return domain.Link{}, err
		}

		if attempt == maxCodeGenerationAttempts-1 {
			return domain.Link{}, errors.Join(
				ErrCodeGenerationExhausted,
				ports.ErrLinkAlreadyExists,
			)
		}
	}

	if request.IdempotencyKey != "" && c.idempotencyStore != nil {
		if err := c.idempotencyStore.Save(ctx, request.OwnerID, request.IdempotencyKey, link); err != nil {
			return domain.Link{}, err
		}
	}

	return link, nil
}
