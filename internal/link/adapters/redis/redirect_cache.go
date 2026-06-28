package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"tinyurl/internal/link/domain"
	"tinyurl/internal/link/ports"
)

const redirectKeyPrefix = "tinyurl:redirect:v1:"

var _ ports.RedirectCache = RedirectCache{}

var putIfNewerScript = redisclient.NewScript(`
local current = redis.call("HGET", KEYS[1], "version")
local incoming = ARGV[3]

if current then
	if string.len(current) > string.len(incoming) then
		return 0
	end

	if string.len(current) == string.len(incoming) and current > incoming then
		return 0
	end
end

redis.call(
	"HSET",
	KEYS[1],
	"destination", ARGV[1],
	"status", ARGV[2],
	"version", ARGV[3],
	"expires_at", ARGV[4]
)

redis.call("PEXPIRE", KEYS[1], ARGV[5])

return 1
`)

// RedirectCache stores redirect projections in Redis.
type RedirectCache struct {
	client redisclient.Cmdable
}

func NewRedirectCache(client redisclient.Cmdable) RedirectCache {
	return RedirectCache{
		client: client,
	}
}

func (c RedirectCache) Get(
	ctx context.Context,
	code string,
) (domain.RedirectMapping, error) {
	key := redirectKeyPrefix + code

	fields, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return domain.RedirectMapping{}, fmt.Errorf(
			"read redirect cache key %q: %w",
			key,
			err,
		)
	}

	if len(fields) == 0 {
		return domain.RedirectMapping{}, ports.ErrRedirectCacheMiss
	}

	destination, err := domain.NewDestinationURL(fields["destination"])
	if err != nil {
		return domain.RedirectMapping{}, fmt.Errorf(
			"decode cached destination: %w",
			err,
		)
	}

	status, err := domain.ParseLinkStatus(fields["status"])
	if err != nil {
		return domain.RedirectMapping{}, fmt.Errorf(
			"decode cache status: %w",
			err,
		)
	}

	version, err := strconv.ParseUint(fields["version"], 10, 64)
	if err != nil {
		return domain.RedirectMapping{}, fmt.Errorf(
			"decode cached version: %w",
			err,
		)
	}

	expiresAt, err := parseExpiresAt(fields["expires_at"])
	if err != nil {
		return domain.RedirectMapping{}, err
	}

	mapping, err := domain.NewRedirectMapping(
		code,
		destination,
		status,
		expiresAt,
		version,
	)
	if err != nil {
		return domain.RedirectMapping{}, fmt.Errorf(
			"build cached redirect mapping: %w",
			err,
		)
	}

	return mapping, nil
}

func (c RedirectCache) PutIfNewer(
	ctx context.Context,
	mapping domain.RedirectMapping,
	ttl time.Duration,
) error {
	ttlMilliseconds := ttl.Milliseconds()
	if ttlMilliseconds <= 0 {
		return fmt.Errorf("redirect cache TTL must be at least one millinsecond")
	}

	expiresAt := ""
	if value := mapping.ExpiresAt(); value != nil {
		expiresAt = strconv.FormatInt(value.UnixMilli(), 10)
	}

	key := redirectKeyPrefix + mapping.Code()

	_, err := putIfNewerScript.Run(
		ctx,
		c.client,
		[]string{key},
		mapping.Destination().String(),
		mapping.Status().String(),
		strconv.FormatUint(mapping.Version(), 10),
		expiresAt,
		ttlMilliseconds,
	).Result()
	if err != nil {
		return fmt.Errorf(
			"write redirect cache key %q: %w",
			key,
			err,
		)
	}

	return nil
}

func parseExpiresAt(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	milliseconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("decode cached expiration: %w", err)
	}

	expiresAt := time.UnixMilli(milliseconds).UTC()
	return &expiresAt, nil
}
