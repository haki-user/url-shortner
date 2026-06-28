package application

import (
	"context"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

type linkMutationRefresher interface {
	Refresh(context.Context, domain.Link) error
}

// CacheRefreshingRepository decorates the source-of-truth repository with a
// best-effort cache refresh after successful updates.
type CacheRefreshingRepository struct {
	source    ports.LinkRepository
	refresher linkMutationRefresher
}

func NewCacheRefreshingRepository(
	source ports.LinkRepository,
	refresher linkMutationRefresher,
) CacheRefreshingRepository {
	return CacheRefreshingRepository{
		source:    source,
		refresher: refresher,
	}
}

func (r CacheRefreshingRepository) Insert(
	ctx context.Context,
	link domain.Link,
) error {
	return r.source.Insert(ctx, link)
}

func (r CacheRefreshingRepository) FindByCode(
	ctx context.Context,
	code string,
) (domain.Link, error) {
	return r.source.FindByCode(ctx, code)
}

func (r CacheRefreshingRepository) Update(
	ctx context.Context,
	link domain.Link,
	expectedVersion uint64,
) error {
	if err := r.source.Update(ctx, link, expectedVersion); err != nil {
		return err
	}

	// The durable update already succeeded, so cache failure cannot fail it.
	_ = r.refresher.Refresh(ctx, link)

	return nil
}

var _ ports.LinkRepository = CacheRefreshingRepository{}
