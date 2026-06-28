package application

import (
	"context"
	"errors"
	"strings"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

var ErrInvalidExpectedVersion = errors.New("invalid expected version")

type ChangeLinkStatusRequest struct {
	Code            string
	OwnerID         string
	Status          domain.LinkStatus
	ExpectedVersion uint64
}

type ChangeLinkStatus struct {
	repository ports.LinkRepository
	clock      ports.Clock
}

func NewChangeLinkStatus(
	repository ports.LinkRepository,
	clock ports.Clock,
) ChangeLinkStatus {
	return ChangeLinkStatus{
		repository: repository,
		clock:      clock,
	}
}

func (c ChangeLinkStatus) Execute(
	ctx context.Context,
	request ChangeLinkStatusRequest,
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

	switch request.Status {
	case domain.Active:
		err = link.Reactivate(now)
	case domain.Disabled:
		err = link.Disable(now)
	case domain.Deleted:
		err = link.Delete(now)
	default:
		err = domain.ErrInvalidLinkStatus
	}
	if err != nil {
		return domain.Link{}, err
	}

	if err := c.repository.Update(ctx, link, request.ExpectedVersion); err != nil {
		return domain.Link{}, err
	}

	return link, nil
}
