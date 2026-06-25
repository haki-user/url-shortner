package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

const uniqueViolationCode = "23505"

var _ ports.LinkRepository = (*Repository)(nil)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, link domain.Link) error {
	_, err := r.pool.Exec(
		ctx,
		`insert into links (
			code,
			destination_url,
			owner_id,
			status,
			created_at,
			updated_at,
			expires_at,
			version
		) values ($1, $2, $3, $4, $5, $6, $7, $8)`,
		link.Code(),
		link.Destination().String(),
		link.OwnerID(),
		link.Status().String(),
		link.CreatedAt(),
		link.UpdatedAt(),
		link.ExpiresAt(),
		link.Version(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ports.ErrLinkAlreadyExists
		}

		return err
	}

	return nil
}

func (r *Repository) FindByCode(ctx context.Context, code string) (domain.Link, error) {
	link, err := scanLink(r.pool.QueryRow(
		ctx,
		`select
			code,
			destination_url,
			owner_id,
			status,
			created_at,
			updated_at,
			expires_at,
			version
		from links
		where code = $1`,
		code,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, ports.ErrLinkNotFound
		}

		return domain.Link{}, err
	}

	return link, nil
}

func (r *Repository) Update(ctx context.Context, link domain.Link, expectedVersion uint64) error {
	commandTag, err := r.pool.Exec(
		ctx,
		`update links
		set destination_url = $1,
			status = $2,
			updated_at = $3,
			expires_at = $4,
			version = $5
		where code = $6
			and version = $7`,
		link.Destination().String(),
		link.Status().String(),
		link.UpdatedAt(),
		link.ExpiresAt(),
		link.Version(),
		link.Code(),
		expectedVersion,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 1 {
		return nil
	}

	exists, err := r.linkExists(ctx, link.Code())
	if err != nil {
		return err
	}

	if !exists {
		return ports.ErrLinkNotFound
	}

	return ports.ErrVersionConflict
}

func (r *Repository) linkExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(
		ctx,
		`select exists(select 1 from links where code = $1)`,
		code,
	).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func scanLink(row pgx.Row) (domain.Link, error) {
	var (
		code           string
		destinationRaw string
		ownerID        string
		statusRaw      string
		createdAt      time.Time
		updatedAt      time.Time
		expiresAt      *time.Time
		version        int64
	)

	err := row.Scan(
		&code,
		&destinationRaw,
		&ownerID,
		&statusRaw,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&version,
	)
	if err != nil {
		return domain.Link{}, err
	}

	destination, err := domain.NewDestinationURL(destinationRaw)
	if err != nil {
		return domain.Link{}, err
	}

	status, err := domain.ParseLinkStatus(statusRaw)
	if err != nil {
		return domain.Link{}, err
	}

	if version < 0 {
		return domain.Link{}, domain.ErrZeroVersion
	}

	return domain.RehydrateLink(
		code,
		destination,
		ownerID,
		status,
		createdAt,
		updatedAt,
		expiresAt,
		uint64(version),
	)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
