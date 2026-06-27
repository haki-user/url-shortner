package application

import (
	"context"
	"strings"
	"time"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type ChangeLinkExpirationRequest struct {
	Code            string
	OwnerID         string
	ExpiresAt       *time.Time
	ExpectedVersion uint64
}

type ChangeLinkExpiration struct {
	repository ports.LinkRepository
	clock      ports.Clock
}

func NewChangeLinkExpiration(
	repository ports.LinkRepository,
	clock ports.Clock,
) ChangeLinkExpiration {
	return ChangeLinkExpiration{
		repository: repository,
		clock:      clock,
	}
}

func (c ChangeLinkExpiration) Execute(
	ctx context.Context,
	request ChangeLinkExpirationRequest,
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

	now := c.clock.Now()

	if request.ExpiresAt == nil {
		err = link.ClearExpiration(now)
	} else {
		err = link.SetExpiration(*request.ExpiresAt, now)
	}
	if err != nil {
		return domain.Link{}, err
	}

	if err := c.repository.Update(ctx, link, request.ExpectedVersion); err != nil {
		return domain.Link{}, err
	}

	return link, nil
}
