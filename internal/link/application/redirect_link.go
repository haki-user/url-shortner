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
	resolver ports.LinkResolver
	clock    ports.Clock
}

func NewRedirectLink(
	resolver ports.LinkResolver,
	clock ports.Clock,
) RedirectLink {
	return RedirectLink{
		resolver: resolver,
		clock:    clock,
	}
}

func (r RedirectLink) Execute(
	ctx context.Context,
	request RedirectLinkRequest,
) (RedirectLinkResult, error) {
	mapping, err := r.resolver.Resolve(ctx, request.Code)
	if err != nil {
		return RedirectLinkResult{}, err
	}

	if !mapping.CanRedirect(r.clock.Now()) {
		return RedirectLinkResult{}, ErrLinkUnavailable
	}

	return RedirectLinkResult{
		Destination: mapping.Destination().String(),
	}, nil
}
