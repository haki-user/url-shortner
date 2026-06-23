package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyCode              = errors.New("empty code")
	ErrZeroDestination        = errors.New("zero destination")
	ErrEmptyOwnerID           = errors.New("empty owner id")
	ErrZeroCreatedAt          = errors.New("zero created at")
	ErrInvalidExpiresAt       = errors.New("invalid expires at")
	ErrInvalidTransition      = errors.New("invalid transition")
	ErrZeroUpdatedAt          = errors.New("zero updated at")
	ErrUpdateBeforeCreatedAt  = errors.New("update before created at")
	ErrUpdateNotAfterPrevious = errors.New("update not after previous")
	ErrDeletedLink            = errors.New("deleted link")
	ErrUnchangedDestination   = errors.New("unchanged destination")
	ErrUnchangedExpiration    = errors.New("unchanged expiration")
	ErrNoExpiration           = errors.New("no expiration")
)

type Link struct {
	code        string
	destination DestinationURL
	ownerID     string
	status      LinkStatus
	createdAt   time.Time
	updatedAt   time.Time
	expiresAt   *time.Time
	version     uint64
}

func NewLink(
	code string,
	destination DestinationURL,
	ownerID string,
	createdAt time.Time,
	expiresAt *time.Time,
) (Link, error) {
	trimmedCode := strings.TrimSpace(code)
	if trimmedCode == "" {
		return Link{}, ErrEmptyCode
	}

	if destination.IsZero() {
		return Link{}, ErrZeroDestination
	}

	trimmedOwnerID := strings.TrimSpace(ownerID)
	if trimmedOwnerID == "" {
		return Link{}, ErrEmptyOwnerID
	}

	if createdAt.IsZero() {
		return Link{}, ErrZeroCreatedAt
	}

	var expiresAtCopy *time.Time
	if expiresAt != nil {
		if !expiresAt.After(createdAt) {
			return Link{}, ErrInvalidExpiresAt
		}
		copied := *expiresAt
		expiresAtCopy = &copied
	}

	return Link{
		code:        trimmedCode,
		destination: destination,
		ownerID:     trimmedOwnerID,
		status:      Active,
		createdAt:   createdAt,
		updatedAt:   createdAt,
		expiresAt:   expiresAtCopy,
		version:     1,
	}, nil
}

func (l Link) Code() string {
	return l.code
}

func (l Link) Destination() DestinationURL {
	return l.destination
}

func (l Link) OwnerID() string {
	return l.ownerID
}

func (l Link) Status() LinkStatus {
	return l.status
}

func (l Link) CreatedAt() time.Time {
	return l.createdAt
}

func (l Link) UpdatedAt() time.Time {
	return l.updatedAt
}

func (l Link) ExpiresAt() *time.Time {
	if l.expiresAt == nil {
		return nil
	}

	copied := *l.expiresAt
	return &copied
}

func (l Link) IsExpired(now time.Time) bool {
	if l.expiresAt == nil {
		return false
	}

	return !now.Before(*l.expiresAt)
}

func (l Link) CanRedirect(now time.Time) bool {
	if now.IsZero() {
		return false
	}

	return l.status.CanRedirect() && !l.IsExpired(now)
}

func (l Link) Version() uint64 {
	return l.version
}

func (l *Link) Disable(at time.Time) error {
	return l.transitionTo(Disabled, at)
}

func (l *Link) Reactivate(at time.Time) error {
	return l.transitionTo(Active, at)
}

func (l *Link) Delete(at time.Time) error {
	return l.transitionTo(Deleted, at)
}

func (l *Link) transitionTo(next LinkStatus, at time.Time) error {
	if err := l.validateMutationTime(at); err != nil {
		return err
	}

	if !l.status.CanTransitionTo(next) {
		return ErrInvalidTransition
	}

	l.status = next
	l.updatedAt = at
	l.version++

	return nil
}

func (l Link) validateMutationTime(at time.Time) error {
	if at.IsZero() {
		return ErrZeroUpdatedAt
	}

	if at.Before(l.createdAt) {
		return ErrUpdateBeforeCreatedAt
	}

	if !at.After(l.updatedAt) {
		return ErrUpdateNotAfterPrevious
	}

	return nil
}

func (l *Link) UpdateDestination(destination DestinationURL, at time.Time) error {
	if destination.IsZero() {
		return ErrZeroDestination
	}

	if l.status == Deleted {
		return ErrDeletedLink
	}

	if l.destination == destination {
		return ErrUnchangedDestination
	}

	if err := l.validateMutationTime(at); err != nil {
		return err
	}

	l.destination = destination
	l.updatedAt = at
	l.version++

	return nil
}

func (l *Link) SetExpiration(expiresAt time.Time, at time.Time) error {
	if expiresAt.IsZero() {
		return ErrInvalidExpiresAt
	}

	if l.status == Deleted {
		return ErrDeletedLink
	}

	if !expiresAt.After(at) {
		return ErrInvalidExpiresAt
	}

	if l.expiresAt != nil && expiresAt.Equal(*l.expiresAt) {
		return ErrUnchangedExpiration
	}

	if err := l.validateMutationTime(at); err != nil {
		return err
	}

	l.expiresAt = &expiresAt
	l.updatedAt = at
	l.version++

	return nil
}

func (l *Link) ClearExpiration(at time.Time) error {
	if l.status == Deleted {
		return ErrDeletedLink
	}

	if l.expiresAt == nil {
		return ErrNoExpiration
	}

	if err := l.validateMutationTime(at); err != nil {
		return err
	}

	l.expiresAt = nil
	l.updatedAt = at
	l.version++

	return nil
}
