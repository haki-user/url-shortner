package application

import (
	"context"
	"errors"
	"strings"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

var ErrLinkAccessDenied = errors.New("link access denied")

type GetManagedLinkRequest struct {
	Code    string
	OwnerID string
}

type GetManagedLink struct {
	repository ports.LinkRepository
}

func NewGetManagedLink(repository ports.LinkRepository) GetManagedLink {
	return GetManagedLink{repository: repository}
}

func (g GetManagedLink) Execute(
	ctx context.Context,
	request GetManagedLinkRequest,
) (domain.Link, error) {
	ownerID := strings.TrimSpace(request.OwnerID)
	if ownerID == "" {
		return domain.Link{}, ErrLinkAccessDenied
	}

	link, err := g.repository.FindByCode(ctx, strings.TrimSpace(request.Code))
	if err != nil {
		return domain.Link{}, err
	}

	if link.OwnerID() != ownerID {
		return domain.Link{}, ErrLinkAccessDenied
	}

	return link, nil
}
