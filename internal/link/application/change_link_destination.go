package application

import (
	"context"
	"strings"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type ChangeLinkDestinationRequest struct {
	Code            string
	OwnerID         string
	Destination     string
	ExpectedVersion uint64
}

type ChangeLinkDestination struct {
	repository ports.LinkRepository
	clock      ports.Clock
}

func NewChangeLinkDestination(
	repository ports.LinkRepository,
	clock ports.Clock,
) ChangeLinkDestination {
	return ChangeLinkDestination{
		repository: repository,
		clock:      clock,
	}
}

func (c ChangeLinkDestination) Execute(
	ctx context.Context,
	request ChangeLinkDestinationRequest,
) (domain.Link, error) {
	ownerID := strings.TrimSpace(request.OwnerID)
	if ownerID == "" {
		return domain.Link{}, ErrLinkAccessDenied
	}

	if request.ExpectedVersion == 0 {
		return domain.Link{}, ErrInvalidExpectedVersion
	}

	link, err := c.repository.FindByCode(ctx, strings.TrimSpace(request.Code))
	if err != nil {
		return domain.Link{}, err
	}

	if link.OwnerID() != ownerID {
		return domain.Link{}, ErrLinkAccessDenied
	}

	if link.Version() != request.ExpectedVersion {
		return domain.Link{}, ports.ErrVersionConflict
	}

	destination, err := domain.NewDestinationURL(request.Destination)
	if err != nil {
		return domain.Link{}, err
	}

	if err := link.UpdateDestination(destination, c.clock.Now()); err != nil {
		return domain.Link{}, err
	}

	if err := c.repository.Update(ctx, link, request.ExpectedVersion); err != nil {
		return domain.Link{}, err
	}

	return link, nil
}
