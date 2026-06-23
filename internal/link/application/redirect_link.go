package application

import (
	"context"
	"errors"

	"tinyurl/internal/link/ports"
)

var ErrLinkUnavailable = errors.New("link unavailable")

type RedirectLinkRequest struct {
	Code string
}

type RedirectLinkResult struct {
	Destination string
}

type RedirectLink struct {
	repository ports.LinkRepository
	clock      ports.Clock
}

func NewRedirectLink(repository ports.LinkRepository, clock ports.Clock) RedirectLink {
	return RedirectLink{
		repository: repository,
		clock:      clock,
	}
}

func (r RedirectLink) Execute(ctx context.Context, request RedirectLinkRequest) (RedirectLinkResult, error) {
	link, err := r.repository.FindByCode(ctx, request.Code)
	if err != nil {
		return RedirectLinkResult{}, err
	}

	now := r.clock.Now()
	if !link.CanRedirect(now) {
		return RedirectLinkResult{}, ErrLinkUnavailable
	}

	return RedirectLinkResult{
		Destination: link.Destination().String(),
	}, nil
}
