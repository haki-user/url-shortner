package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewLinkCreatesActiveLinkWithoutExpiration(t *testing.T) {
	code := "abc123"
	destination := DestinationURL{value: "https://example.com"}
	ownerID := "owner-1"
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)

	link, err := NewLink(code, destination, ownerID, createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Code() != code {
		t.Fatalf("expected code %q, got %q", code, link.Code())
	}

	if link.Destination() != destination {
		t.Fatalf("expected destination %q, got %q", destination.String(), link.Destination())
	}

	if link.OwnerID() != ownerID {
		t.Fatalf("expected ownerID %q, got %q", ownerID, link.OwnerID())
	}

	if link.Status() != Active {
		t.Fatalf("expected status %v, got %v", Active, link.status)
	}

	if link.CreatedAt() != createdAt {
		t.Fatalf("expected createdAt %v, got %v", createdAt, link.CreatedAt())
	}

	if link.UpdatedAt() != createdAt {
		t.Fatalf("expected updatedAt %v, got %v", createdAt, link.UpdatedAt())
	}

	if link.ExpiresAt() != nil {
		t.Fatalf("expected nil expiresAt, got %v", link.ExpiresAt())
	}

	if link.Version() != 1 {
		t.Fatalf("expected version %d, got %d", uint(1), link.Version())
	}
}

func TestNewLinkCreatesActiveLinkWithExpiration(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)

	link, err := NewLink("abc123", destination, "owner-1", createdAt, &expiresAt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	if !actualExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiresAt %v, got %v", expiresAt, actualExpiresAt)
	}
}

func TestNewLinkRejectsInvalidInput(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	validExpiresAt := createdAt.Add(24 * time.Hour)
	sameAsCreatedAt := createdAt
	beforeCreatedAt := createdAt.Add(-time.Second)

	tests := []struct {
		name        string
		code        string
		destination DestinationURL
		ownerID     string
		createdAt   time.Time
		expiresAt   *time.Time
		expectedErr error
	}{
		{
			name:        "empty code",
			code:        "",
			destination: destination,
			ownerID:     "owner-1",
			createdAt:   createdAt,
			expiresAt:   &validExpiresAt,
			expectedErr: ErrEmptyCode,
		},
		{
			name:        "whitespace-only code",
			code:        "   ",
			destination: destination,
			ownerID:     "owner-1",
			createdAt:   createdAt,
			expiresAt:   &validExpiresAt,
			expectedErr: ErrEmptyCode,
		},
		{
			name:        "zero destination",
			code:        "abc123",
			destination: DestinationURL{},
			ownerID:     "owner-1",
			createdAt:   createdAt,
			expiresAt:   &validExpiresAt,
			expectedErr: ErrZeroDestination,
		},
		{
			name:        "empty owner ID",
			code:        "abc123",
			destination: destination,
			ownerID:     "",
			createdAt:   createdAt,
			expiresAt:   &validExpiresAt,
			expectedErr: ErrEmptyOwnerID,
		},
		{
			name:        "whitespace-only owner ID",
			code:        "abc123",
			destination: destination,
			ownerID:     "   ",
			createdAt:   createdAt,
			expiresAt:   &validExpiresAt,
			expectedErr: ErrEmptyOwnerID,
		},
		{
			name:        "zero createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			createdAt:   time.Time{},
			expiresAt:   nil,
			expectedErr: ErrZeroCreatedAt,
		},
		{
			name:        "expiresAt equal to createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			createdAt:   createdAt,
			expiresAt:   &sameAsCreatedAt,
			expectedErr: ErrInvalidExpiresAt,
		},
		{
			name:        "expiresAt before createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			createdAt:   createdAt,
			expiresAt:   &beforeCreatedAt,
			expectedErr: ErrInvalidExpiresAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink(tt.code, tt.destination, tt.ownerID, tt.createdAt, tt.expiresAt)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if link.Version() != 0 {
				t.Fatalf("expected zero-value link on error, got version %d", link.Version())
			}
		})
	}
}

func TestLinkExpiresAtReturnsCopy(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)

	link, err := NewLink("abc-123", destination, "owner-1", createdAt, &expiresAt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	returnedExpiresAt := link.ExpiresAt()
	if returnedExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	*returnedExpiresAt = returnedExpiresAt.Add(48 * time.Hour)

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatalf("expected expiresAt, got nil")
	}

	if !actualExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected internal expiresAt to remain %v, got %v", expiresAt, actualExpiresAt)
	}
}

func TestNewLinkCopiesExpiresAtInput(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)

	link, err := NewLink("abc123", destination, "owner-1", createdAt, &expiresAt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expiresAt = expiresAt.Add(48 * time.Hour)

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	expectedExpiresAt := createdAt.Add(24 * time.Hour)
	if !actualExpiresAt.Equal(expectedExpiresAt) {
		t.Fatalf("expected expiresAt %v, got %v", expectedExpiresAt, *actualExpiresAt)
	}
}

func TestNewLinkTrimsCodeAndOwnerID(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)

	link, err := NewLink(" abc123 ", destination, " owner-1 ", createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Code() != "abc123" {
		t.Fatalf("expected trimmed code %q, got %q", "abc123", link.Code())
	}

	if link.OwnerID() != "owner-1" {
		t.Fatalf("expected trimmed ownerID %q, got %q", "owner-1", link.OwnerID())
	}
}

func TestLinkIsExpired(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name      string
		expiresAt *time.Time
		now       time.Time
		expected  bool
	}{
		{
			name:      "not expired when no expiration exists",
			expiresAt: nil,
			now:       createdAt,
			expected:  false,
		},
		{
			name:      "not expired before expiration",
			expiresAt: &expiresAt,
			now:       expiresAt.Add(-time.Nanosecond),
			expected:  false,
		},
		{
			name:      "expired exactly at expiration",
			expiresAt: &expiresAt,
			now:       expiresAt,
			expected:  true,
		},
		{
			name:      "expired after expiration",
			expiresAt: &expiresAt,
			now:       expiresAt.Add(time.Nanosecond),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, tt.expiresAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			actual := link.IsExpired(tt.now)
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestLinkCanRedirect(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name      string
		status    LinkStatus
		expiresAt *time.Time
		now       time.Time
		expected  bool
	}{
		{
			name:      "active with no expiration can redirect",
			status:    Active,
			expiresAt: nil,
			now:       createdAt,
			expected:  true,
		},
		{
			name:      "active before expiration can redirect",
			status:    Active,
			expiresAt: &expiresAt,
			now:       expiresAt.Add(-time.Nanosecond),
			expected:  true,
		},
		{
			name:      "active exactly at expiration cannot redirect",
			status:    Active,
			expiresAt: &expiresAt,
			now:       expiresAt,
			expected:  false,
		},
		{
			name:      "active after expiration cannot redirect",
			status:    Active,
			expiresAt: &expiresAt,
			now:       expiresAt.Add(time.Nanosecond),
			expected:  false,
		},
		{
			name:      "disabled cannot redirect",
			status:    Disabled,
			expiresAt: nil,
			now:       createdAt,
			expected:  false,
		},
		{
			name:      "deleted cannot redirect",
			status:    Deleted,
			expiresAt: nil,
			now:       createdAt,
			expected:  false,
		},
		{
			name:      "unknown cannot redirect",
			status:    Unknown,
			expiresAt: nil,
			now:       createdAt,
			expected:  false,
		},
		{
			name:      "zero now cannot redirect",
			status:    Active,
			expiresAt: nil,
			now:       time.Time{},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, tt.expiresAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			link.status = tt.status

			actual := link.CanRedirect(tt.now)
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestLinkLifecycleTransitions(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name           string
		initialStatus  LinkStatus
		mutate         func(*Link, time.Time) error
		expectedStatus LinkStatus
		expectedErr    error
	}{
		{
			name:           "disable active link",
			initialStatus:  Active,
			mutate:         (*Link).Disable,
			expectedStatus: Disabled,
			expectedErr:    nil,
		},
		{
			name:           "disable disabled link is invalid",
			initialStatus:  Disabled,
			mutate:         (*Link).Disable,
			expectedStatus: Disabled,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "disable deleted link is invalid",
			initialStatus:  Deleted,
			mutate:         (*Link).Disable,
			expectedStatus: Deleted,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "disable unknown link is invalid",
			initialStatus:  Unknown,
			mutate:         (*Link).Disable,
			expectedStatus: Unknown,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "reactivate active link is invalid",
			initialStatus:  Active,
			mutate:         (*Link).Reactivate,
			expectedStatus: Active,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "reactivate disabled link",
			initialStatus:  Disabled,
			mutate:         (*Link).Reactivate,
			expectedStatus: Active,
			expectedErr:    nil,
		},
		{
			name:           "reactivate deleted link is invalid",
			initialStatus:  Deleted,
			mutate:         (*Link).Reactivate,
			expectedStatus: Deleted,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "reactivate unknown link is invalid",
			initialStatus:  Unknown,
			mutate:         (*Link).Reactivate,
			expectedStatus: Unknown,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "delete active link",
			initialStatus:  Active,
			mutate:         (*Link).Delete,
			expectedStatus: Deleted,
			expectedErr:    nil,
		},
		{
			name:           "delete disabled link",
			initialStatus:  Disabled,
			mutate:         (*Link).Delete,
			expectedStatus: Deleted,
			expectedErr:    nil,
		},
		{
			name:           "delete deleted link is invalid",
			initialStatus:  Deleted,
			mutate:         (*Link).Delete,
			expectedStatus: Deleted,
			expectedErr:    ErrInvalidTransition,
		},
		{
			name:           "delete unknown link is invalid",
			initialStatus:  Unknown,
			mutate:         (*Link).Delete,
			expectedStatus: Unknown,
			expectedErr:    ErrInvalidTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			link.status = tt.initialStatus

			err = tt.mutate(&link, updatedAt)

			if tt.expectedErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if link.Status() != tt.expectedStatus {
					t.Fatalf("expected status %v, got %v", tt.expectedStatus, link.Status())
				}

				if !link.UpdatedAt().Equal(updatedAt) {
					t.Fatalf("expected updatedAt %v, got %v", updatedAt, link.UpdatedAt())
				}

				if link.Version() != 2 {
					t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
				}

				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if link.Status() != tt.expectedStatus {
				t.Fatalf("expected status to remain %v, got %v", tt.expectedStatus, link.Status())
			}

			if !link.UpdatedAt().Equal(createdAt) {
				t.Fatalf("expected updatedAt to remain %v, got %v", createdAt, link.UpdatedAt())
			}

			if link.Version() != 1 {
				t.Fatalf("expected version to remain %d, got %d", uint64(1), link.Version())
			}
		})
	}
}

func TestLinkLifecycleRejectsInvalidMutationTimes(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name        string
		at          time.Time
		expectedErr error
	}{
		{
			name:        "zero mutation time",
			at:          time.Time{},
			expectedErr: ErrZeroUpdatedAt,
		},
		{
			name:        "mutation time before createdAt",
			at:          createdAt.Add(-time.Nanosecond),
			expectedErr: ErrUpdateBeforeCreatedAt,
		},
		{
			name:        "mutation time equal to previous updatedAt",
			at:          createdAt,
			expectedErr: ErrUpdateNotAfterPrevious,
		},
		{
			name:        "mutation time after createdAt but before previous updatedAt",
			at:          createdAt.Add(30 * time.Minute),
			expectedErr: ErrUpdateNotAfterPrevious,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if tt.name == "mutation time after createdAt but before previous updatedAt" {
				err = link.Disable(createdAt.Add(time.Hour))
				if err != nil {
					t.Fatalf("expected setup mutation to succeed, got %v", err)
				}

				err = link.Reactivate(tt.at)
			} else {
				err = link.Disable(tt.at)
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestLinkLifecycleIncrementsVersionOnSuccessfulMutations(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	destination := DestinationURL{value: "https://example.com"}

	link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Version() != 1 {
		t.Fatalf("expected initial version %d, got %d", uint64(1), link.Version())
	}

	disabledAt := createdAt.Add(time.Hour)
	if err := link.Disable(disabledAt); err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	if link.Version() != 2 {
		t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
	}

	reactivatedAt := disabledAt.Add(time.Hour)
	if err := link.Reactivate(reactivatedAt); err != nil {
		t.Fatalf("expected reactivate to succeed, got %v", err)
	}

	if link.Version() != 3 {
		t.Fatalf("expected version %d, got %d", uint64(3), link.Version())
	}

	deletedAt := reactivatedAt.Add(time.Hour)
	if err := link.Delete(deletedAt); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	if link.Version() != 4 {
		t.Fatalf("expected version %d, got %d", uint64(4), link.Version())
	}
}

func TestLinkLifecycleFailureChangesNothing(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name   string
		setup  func(*Link)
		mutate func(*Link) error
	}{
		{
			name:  "invalid transition changes nothing",
			setup: func(link *Link) {},
			mutate: func(link *Link) error {
				return link.Reactivate(createdAt.Add(time.Hour))
			},
		},
		{
			name:  "zero mutation time changes nothing",
			setup: func(link *Link) {},
			mutate: func(link *Link) error {
				return link.Disable(time.Time{})
			},
		},
		{
			name: "stale mutation time changes nothing",
			setup: func(link *Link) {
				if err := link.Disable(createdAt.Add(time.Hour)); err != nil {
					t.Fatalf("expected setup mutation to succeed, got %v", err)
				}
			},
			mutate: func(link *Link) error {
				return link.Reactivate(createdAt.Add(30 * time.Minute))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			tt.setup(&link)

			originalCode := link.Code()
			originalDestination := link.Destination()
			originalOwnerID := link.OwnerID()
			originalStatus := link.Status()
			originalCreatedAt := link.CreatedAt()
			originalUpdatedAt := link.UpdatedAt()
			originalExpiresAt := link.ExpiresAt()
			originalVersion := link.Version()

			err = tt.mutate(&link)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if link.Code() != originalCode {
				t.Fatalf("expected code to remain %q, got %q", originalCode, link.Code())
			}

			if link.Destination() != originalDestination {
				t.Fatalf("expected destination to remain %q, got %q", originalDestination.String(), link.Destination().String())
			}

			if link.OwnerID() != originalOwnerID {
				t.Fatalf("expected ownerID to remain %q, got %q", originalOwnerID, link.OwnerID())
			}

			if link.Status() != originalStatus {
				t.Fatalf("expected status to remain %v, got %v", originalStatus, link.Status())
			}

			if !link.CreatedAt().Equal(originalCreatedAt) {
				t.Fatalf("expected createdAt to remain %v, got %v", originalCreatedAt, link.CreatedAt())
			}

			if !link.UpdatedAt().Equal(originalUpdatedAt) {
				t.Fatalf("expected updatedAt to remain %v, got %v", originalUpdatedAt, link.UpdatedAt())
			}

			actualExpiresAt := link.ExpiresAt()
			if originalExpiresAt == nil && actualExpiresAt != nil {
				t.Fatalf("expected expiresAt to remain nil, got %v", actualExpiresAt)
			}

			if originalExpiresAt != nil {
				if actualExpiresAt == nil {
					t.Fatal("expected expiresAt to remain non-nil, got nil")
				}

				if !actualExpiresAt.Equal(*originalExpiresAt) {
					t.Fatalf("expected expiresAt to remain %v, got %v", *originalExpiresAt, *actualExpiresAt)
				}
			}

			if link.Version() != originalVersion {
				t.Fatalf("expected version to remain %d, got %d", originalVersion, link.Version())
			}
		})
	}
}

func TestLinkUpdateDestinationSucceedsForActiveAndDisabledLinks(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	originalDestination := DestinationURL{value: "https://example.com"}
	newDestination := DestinationURL{value: "https://new.example.com"}

	tests := []struct {
		name          string
		initialStatus LinkStatus
	}{
		{
			name:          "active link can update destination",
			initialStatus: Active,
		},
		{
			name:          "disabled link can update destination",
			initialStatus: Disabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", originalDestination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			link.status = tt.initialStatus

			err = link.UpdateDestination(newDestination, updatedAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if link.Destination() != newDestination {
				t.Fatalf("expected destination %q, got %q", newDestination.String(), link.Destination().String())
			}

			if link.Status() != tt.initialStatus {
				t.Fatalf("expected status to remain %v, got %v", tt.initialStatus, link.Status())
			}

			if !link.UpdatedAt().Equal(updatedAt) {
				t.Fatalf("expected updatedAt %v, got %v", updatedAt, link.UpdatedAt())
			}

			if link.Version() != 2 {
				t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
			}
		})
	}
}

func TestLinkUpdateDestinationRejectsInvalidUpdates(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	validUpdatedAt := createdAt.Add(time.Hour)
	originalDestination := DestinationURL{value: "https://example.com"}
	newDestination := DestinationURL{value: "https://new.example.com"}

	tests := []struct {
		name        string
		setup       func(*testing.T, *Link)
		destination DestinationURL
		at          time.Time
		expectedErr error
	}{
		{
			name:        "zero destination",
			setup:       func(t *testing.T, link *Link) {},
			destination: DestinationURL{},
			at:          validUpdatedAt,
			expectedErr: ErrZeroDestination,
		},
		{
			name: "zero destination is rejected before deleted link",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
			},
			destination: DestinationURL{},
			at:          validUpdatedAt,
			expectedErr: ErrZeroDestination,
		},
		{
			name: "deleted link",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
			},
			destination: newDestination,
			at:          validUpdatedAt,
			expectedErr: ErrDeletedLink,
		},
		{
			name: "deleted link is rejected before unchanged destination",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
			},
			destination: originalDestination,
			at:          validUpdatedAt,
			expectedErr: ErrDeletedLink,
		},
		{
			name:        "unchanged destination",
			setup:       func(t *testing.T, link *Link) {},
			destination: originalDestination,
			at:          validUpdatedAt,
			expectedErr: ErrUnchangedDestination,
		},
		{
			name:        "zero update time",
			setup:       func(t *testing.T, link *Link) {},
			destination: newDestination,
			at:          time.Time{},
			expectedErr: ErrZeroUpdatedAt,
		},
		{
			name:        "update time before createdAt",
			setup:       func(t *testing.T, link *Link) {},
			destination: newDestination,
			at:          createdAt.Add(-time.Nanosecond),
			expectedErr: ErrUpdateBeforeCreatedAt,
		},
		{
			name:        "update time equal to previous updatedAt",
			setup:       func(t *testing.T, link *Link) {},
			destination: newDestination,
			at:          createdAt,
			expectedErr: ErrUpdateNotAfterPrevious,
		},
		{
			name: "update time before previous updatedAt",
			setup: func(t *testing.T, link *Link) {
				err := link.Disable(createdAt.Add(time.Hour))
				if err != nil {
					t.Fatalf("expected setup mutation to succeed, got %v", err)
				}
			},
			destination: newDestination,
			at:          createdAt.Add(30 * time.Minute),
			expectedErr: ErrUpdateNotAfterPrevious,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", originalDestination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			tt.setup(t, &link)

			originalCode := link.Code()
			originalDestinationBeforeUpdate := link.Destination()
			originalOwnerID := link.OwnerID()
			originalStatus := link.Status()
			originalCreatedAt := link.CreatedAt()
			originalUpdatedAt := link.UpdatedAt()
			originalExpiresAt := link.ExpiresAt()
			originalVersion := link.Version()

			err = link.UpdateDestination(tt.destination, tt.at)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if link.Code() != originalCode {
				t.Fatalf("expected code to remain %q, got %q", originalCode, link.Code())
			}

			if link.Destination() != originalDestinationBeforeUpdate {
				t.Fatalf(
					"expected destination to remain %q, got %q",
					originalDestinationBeforeUpdate.String(),
					link.Destination().String(),
				)
			}

			if link.OwnerID() != originalOwnerID {
				t.Fatalf("expected ownerID to remain %q, got %q", originalOwnerID, link.OwnerID())
			}

			if link.Status() != originalStatus {
				t.Fatalf("expected status to remain %v, got %v", originalStatus, link.Status())
			}

			if !link.CreatedAt().Equal(originalCreatedAt) {
				t.Fatalf("expected createdAt to remain %v, got %v", originalCreatedAt, link.CreatedAt())
			}

			if !link.UpdatedAt().Equal(originalUpdatedAt) {
				t.Fatalf("expected updatedAt to remain %v, got %v", originalUpdatedAt, link.UpdatedAt())
			}

			actualExpiresAt := link.ExpiresAt()
			if originalExpiresAt == nil && actualExpiresAt != nil {
				t.Fatalf("expected expiresAt to remain nil, got %v", actualExpiresAt)
			}

			if originalExpiresAt != nil {
				if actualExpiresAt == nil {
					t.Fatal("expected expiresAt to remain non-nil, got nil")
				}

				if !actualExpiresAt.Equal(*originalExpiresAt) {
					t.Fatalf("expected expiresAt to remain %v, got %v", *originalExpiresAt, *actualExpiresAt)
				}
			}

			if link.Version() != originalVersion {
				t.Fatalf("expected version to remain %d, got %d", originalVersion, link.Version())
			}
		})
	}
}

func TestLinkUpdateDestinationIncrementsVersionOnSuccess(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	originalDestination := DestinationURL{value: "https://example.com"}
	firstDestination := DestinationURL{value: "https://first.example.com"}
	secondDestination := DestinationURL{value: "https://second.example.com"}

	link, err := NewLink("abc123", originalDestination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Version() != 1 {
		t.Fatalf("expected initial version %d, got %d", uint64(1), link.Version())
	}

	firstUpdateAt := createdAt.Add(time.Hour)
	err = link.UpdateDestination(firstDestination, firstUpdateAt)
	if err != nil {
		t.Fatalf("expected first update to succeed, got %v", err)
	}

	if link.Version() != 2 {
		t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
	}

	secondUpdateAt := firstUpdateAt.Add(time.Hour)
	err = link.UpdateDestination(secondDestination, secondUpdateAt)
	if err != nil {
		t.Fatalf("expected second update to succeed, got %v", err)
	}

	if link.Version() != 3 {
		t.Fatalf("expected version %d, got %d", uint64(3), link.Version())
	}
}

func TestLinkSetExpirationSucceedsForActiveAndDisabledLinks(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updateAt := createdAt.Add(time.Hour)
	expiresAt := updateAt.Add(24 * time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name          string
		initialStatus LinkStatus
	}{
		{
			name:          "active link can set expiration",
			initialStatus: Active,
		},
		{
			name:          "disabled link can set expiration",
			initialStatus: Disabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			link.status = tt.initialStatus

			err = link.SetExpiration(expiresAt, updateAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			actualExpiresAt := link.ExpiresAt()
			if actualExpiresAt == nil {
				t.Fatal("expected expiration, got nil")
			}

			if !actualExpiresAt.Equal(expiresAt) {
				t.Fatalf("expected expiration %v, got %v", expiresAt, *actualExpiresAt)
			}

			if link.Status() != tt.initialStatus {
				t.Fatalf("expected status to remain %v, got %v", tt.initialStatus, link.Status())
			}

			if !link.UpdatedAt().Equal(updateAt) {
				t.Fatalf("expected updatedAt %v, got %v", updateAt, link.UpdatedAt())
			}

			if link.Version() != 2 {
				t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
			}
		})
	}
}

func TestLinkClearExpirationSucceedsForActiveAndDisabledLinks(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	initialExpiresAt := createdAt.Add(24 * time.Hour)
	clearAt := createdAt.Add(time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name          string
		initialStatus LinkStatus
	}{
		{
			name:          "active link can clear expiration",
			initialStatus: Active,
		},
		{
			name:          "disabled link can clear expiration",
			initialStatus: Disabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, &initialExpiresAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			link.status = tt.initialStatus

			err = link.ClearExpiration(clearAt)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if link.ExpiresAt() != nil {
				t.Fatalf("expected expiration to be nil, got %v", link.ExpiresAt())
			}

			if link.Status() != tt.initialStatus {
				t.Fatalf("expected status to remain %v, got %v", tt.initialStatus, link.Status())
			}

			if !link.UpdatedAt().Equal(clearAt) {
				t.Fatalf("expected updatedAt %v, got %v", clearAt, link.UpdatedAt())
			}

			if link.Version() != 2 {
				t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
			}
		})
	}
}

func TestLinkSetExpirationRejectsInvalidUpdates(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updateAt := createdAt.Add(time.Hour)
	existingExpiresAt := createdAt.Add(24 * time.Hour)
	newExpiresAt := updateAt.Add(48 * time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name        string
		setup       func(*testing.T, *Link)
		expiresAt   time.Time
		at          time.Time
		expectedErr error
	}{
		{
			name:        "zero expiration",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   time.Time{},
			at:          updateAt,
			expectedErr: ErrInvalidExpiresAt,
		},
		{
			name:        "expiration before update time",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   updateAt.Add(-time.Nanosecond),
			at:          updateAt,
			expectedErr: ErrInvalidExpiresAt,
		},
		{
			name:        "expiration equal to update time",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   updateAt,
			at:          updateAt,
			expectedErr: ErrInvalidExpiresAt,
		},
		{
			name: "deleted link",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
			},
			expiresAt:   newExpiresAt,
			at:          updateAt,
			expectedErr: ErrDeletedLink,
		},
		{
			name: "unchanged expiration",
			setup: func(t *testing.T, link *Link) {
				copied := existingExpiresAt
				link.expiresAt = &copied
			},
			expiresAt:   existingExpiresAt,
			at:          updateAt,
			expectedErr: ErrUnchangedExpiration,
		},
		{
			name:        "zero update time",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   newExpiresAt,
			at:          time.Time{},
			expectedErr: ErrZeroUpdatedAt,
		},
		{
			name:        "update time before createdAt",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   newExpiresAt,
			at:          createdAt.Add(-time.Nanosecond),
			expectedErr: ErrUpdateBeforeCreatedAt,
		},
		{
			name:        "update time equal to previous updatedAt",
			setup:       func(t *testing.T, link *Link) {},
			expiresAt:   newExpiresAt,
			at:          createdAt,
			expectedErr: ErrUpdateNotAfterPrevious,
		},
		{
			name: "update time before previous updatedAt",
			setup: func(t *testing.T, link *Link) {
				err := link.Disable(createdAt.Add(time.Hour))
				if err != nil {
					t.Fatalf("expected setup mutation to succeed, got %v", err)
				}
			},
			expiresAt:   newExpiresAt,
			at:          createdAt.Add(30 * time.Minute),
			expectedErr: ErrUpdateNotAfterPrevious,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			tt.setup(t, &link)

			originalCode := link.Code()
			originalDestination := link.Destination()
			originalOwnerID := link.OwnerID()
			originalStatus := link.Status()
			originalCreatedAt := link.CreatedAt()
			originalUpdatedAt := link.UpdatedAt()
			originalExpiresAt := link.ExpiresAt()
			originalVersion := link.Version()

			err = link.SetExpiration(tt.expiresAt, tt.at)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			assertLinkStateUnchanged(
				t,
				link,
				originalCode,
				originalDestination,
				originalOwnerID,
				originalStatus,
				originalCreatedAt,
				originalUpdatedAt,
				originalExpiresAt,
				originalVersion,
			)
		})
	}
}

func TestLinkClearExpirationRejectsInvalidUpdates(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	existingExpiresAt := createdAt.Add(24 * time.Hour)
	clearAt := createdAt.Add(time.Hour)
	destination := DestinationURL{value: "https://example.com"}

	tests := []struct {
		name        string
		setup       func(*testing.T, *Link)
		at          time.Time
		expectedErr error
	}{
		{
			name: "deleted link with expiration",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
				copied := existingExpiresAt
				link.expiresAt = &copied
			},
			at:          clearAt,
			expectedErr: ErrDeletedLink,
		},
		{
			name: "deleted link without expiration is rejected before missing expiration",
			setup: func(t *testing.T, link *Link) {
				link.status = Deleted
			},
			at:          clearAt,
			expectedErr: ErrDeletedLink,
		},
		{
			name:        "no expiration",
			setup:       func(t *testing.T, link *Link) {},
			at:          clearAt,
			expectedErr: ErrNoExpiration,
		},
		{
			name: "zero update time",
			setup: func(t *testing.T, link *Link) {
				copied := existingExpiresAt
				link.expiresAt = &copied
			},
			at:          time.Time{},
			expectedErr: ErrZeroUpdatedAt,
		},
		{
			name: "update time before createdAt",
			setup: func(t *testing.T, link *Link) {
				copied := existingExpiresAt
				link.expiresAt = &copied
			},
			at:          createdAt.Add(-time.Nanosecond),
			expectedErr: ErrUpdateBeforeCreatedAt,
		},
		{
			name: "update time equal to previous updatedAt",
			setup: func(t *testing.T, link *Link) {
				copied := existingExpiresAt
				link.expiresAt = &copied
			},
			at:          createdAt,
			expectedErr: ErrUpdateNotAfterPrevious,
		},
		{
			name: "update time before previous updatedAt",
			setup: func(t *testing.T, link *Link) {
				copied := existingExpiresAt
				link.expiresAt = &copied

				err := link.Disable(createdAt.Add(time.Hour))
				if err != nil {
					t.Fatalf("expected setup mutation to succeed, got %v", err)
				}
			},
			at:          createdAt.Add(30 * time.Minute),
			expectedErr: ErrUpdateNotAfterPrevious,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			tt.setup(t, &link)

			originalCode := link.Code()
			originalDestination := link.Destination()
			originalOwnerID := link.OwnerID()
			originalStatus := link.Status()
			originalCreatedAt := link.CreatedAt()
			originalUpdatedAt := link.UpdatedAt()
			originalExpiresAt := link.ExpiresAt()
			originalVersion := link.Version()

			err = link.ClearExpiration(tt.at)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			assertLinkStateUnchanged(
				t,
				link,
				originalCode,
				originalDestination,
				originalOwnerID,
				originalStatus,
				originalCreatedAt,
				originalUpdatedAt,
				originalExpiresAt,
				originalVersion,
			)
		})
	}
}

func TestLinkExpirationMutationsIncrementVersionOnSuccess(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	destination := DestinationURL{value: "https://example.com"}

	link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Version() != 1 {
		t.Fatalf("expected initial version %d, got %d", uint64(1), link.Version())
	}

	setAt := createdAt.Add(time.Hour)
	expiresAt := setAt.Add(24 * time.Hour)

	err = link.SetExpiration(expiresAt, setAt)
	if err != nil {
		t.Fatalf("expected set expiration to succeed, got %v", err)
	}

	if link.Version() != 2 {
		t.Fatalf("expected version %d, got %d", uint64(2), link.Version())
	}

	clearAt := setAt.Add(time.Hour)

	err = link.ClearExpiration(clearAt)
	if err != nil {
		t.Fatalf("expected clear expiration to succeed, got %v", err)
	}

	if link.Version() != 3 {
		t.Fatalf("expected version %d, got %d", uint64(3), link.Version())
	}
}

func TestLinkSetExpirationCopiesInput(t *testing.T) {
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	setAt := createdAt.Add(time.Hour)
	expiresAt := setAt.Add(24 * time.Hour)
	originalExpiresAt := expiresAt
	destination := DestinationURL{value: "https://example.com"}

	link, err := NewLink("abc123", destination, "owner-1", createdAt, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = link.SetExpiration(expiresAt, setAt)
	if err != nil {
		t.Fatalf("expected set expiration to succeed, got %v", err)
	}

	expiresAt = expiresAt.Add(48 * time.Hour)

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiration, got nil")
	}

	if !actualExpiresAt.Equal(originalExpiresAt) {
		t.Fatalf("expected expiration %v, got %v", originalExpiresAt, *actualExpiresAt)
	}
}

func TestRehydrateLinkRestoresPersistedState(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	expiresAt := createdAt.Add(24 * time.Hour)

	link, err := RehydrateLink(
		"abc123",
		destination,
		"owner-1",
		Disabled,
		createdAt,
		updatedAt,
		&expiresAt,
		7,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Code() != "abc123" {
		t.Fatalf("expected code %q, got %q", "abc123", link.Code())
	}

	if link.Destination() != destination {
		t.Fatalf("expected destination %q, got %q", destination.String(), link.Destination().String())
	}

	if link.OwnerID() != "owner-1" {
		t.Fatalf("expected owner ID %q, got %q", "owner-1", link.OwnerID())
	}

	if link.Status() != Disabled {
		t.Fatalf("expected status %v, got %v", Disabled, link.Status())
	}

	if !link.CreatedAt().Equal(createdAt) {
		t.Fatalf("expected createdAt %v, got %v", createdAt, link.CreatedAt())
	}

	if !link.UpdatedAt().Equal(updatedAt) {
		t.Fatalf("expected updatedAt %v, got %v", updatedAt, link.UpdatedAt())
	}

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	if !actualExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiresAt %v, got %v", expiresAt, *actualExpiresAt)
	}

	if link.Version() != 7 {
		t.Fatalf("expected version %d, got %d", uint64(7), link.Version())
	}
}

func TestRehydrateLinkAllowsUpdatedAtEqualCreatedAt(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)

	link, err := RehydrateLink(
		"abc123",
		destination,
		"owner-1",
		Active,
		createdAt,
		createdAt,
		nil,
		1,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !link.UpdatedAt().Equal(createdAt) {
		t.Fatalf("expected updatedAt %v, got %v", createdAt, link.UpdatedAt())
	}
}

func TestRehydrateLinkCopiesExpiresAtInput(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	expiresAt := createdAt.Add(24 * time.Hour)
	originalExpiresAt := expiresAt

	link, err := RehydrateLink(
		"abc123",
		destination,
		"owner-1",
		Active,
		createdAt,
		updatedAt,
		&expiresAt,
		2,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expiresAt = expiresAt.Add(48 * time.Hour)

	actualExpiresAt := link.ExpiresAt()
	if actualExpiresAt == nil {
		t.Fatal("expected expiresAt, got nil")
	}

	if !actualExpiresAt.Equal(originalExpiresAt) {
		t.Fatalf("expected expiresAt %v, got %v", originalExpiresAt, *actualExpiresAt)
	}
}

func TestRehydrateLinkRejectsInvalidPersistedState(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	validExpiresAt := createdAt.Add(24 * time.Hour)
	invalidExpiresAt := createdAt

	tests := []struct {
		name        string
		code        string
		destination DestinationURL
		ownerID     string
		status      LinkStatus
		createdAt   time.Time
		updatedAt   time.Time
		expiresAt   *time.Time
		version     uint64
		expectedErr error
	}{
		{
			name:        "empty code",
			code:        "",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &validExpiresAt,
			version:     1,
			expectedErr: ErrEmptyCode,
		},
		{
			name:        "zero destination",
			code:        "abc123",
			destination: DestinationURL{},
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &validExpiresAt,
			version:     1,
			expectedErr: ErrZeroDestination,
		},
		{
			name:        "empty owner ID",
			code:        "abc123",
			destination: destination,
			ownerID:     "",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &validExpiresAt,
			version:     1,
			expectedErr: ErrEmptyOwnerID,
		},
		{
			name:        "invalid status",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Unknown,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &validExpiresAt,
			version:     1,
			expectedErr: ErrInvalidLinkStatus,
		},
		{
			name:        "zero createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   time.Time{},
			updatedAt:   updatedAt,
			expiresAt:   nil,
			version:     1,
			expectedErr: ErrZeroCreatedAt,
		},
		{
			name:        "zero updatedAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   time.Time{},
			expiresAt:   nil,
			version:     1,
			expectedErr: ErrZeroUpdatedAt,
		},
		{
			name:        "updatedAt before createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   createdAt.Add(-time.Nanosecond),
			expiresAt:   nil,
			version:     1,
			expectedErr: ErrUpdateBeforeCreatedAt,
		},
		{
			name:        "expiresAt not after createdAt",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &invalidExpiresAt,
			version:     1,
			expectedErr: ErrInvalidExpiresAt,
		},
		{
			name:        "zero version",
			code:        "abc123",
			destination: destination,
			ownerID:     "owner-1",
			status:      Active,
			createdAt:   createdAt,
			updatedAt:   updatedAt,
			expiresAt:   &validExpiresAt,
			version:     0,
			expectedErr: ErrZeroVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, err := RehydrateLink(
				tt.code,
				tt.destination,
				tt.ownerID,
				tt.status,
				tt.createdAt,
				tt.updatedAt,
				tt.expiresAt,
				tt.version,
			)
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if link.Version() != 0 {
				t.Fatalf("expected zero-value link, got version %d", link.Version())
			}
		})
	}
}

func TestRehydrateLinkTrimsCodeAndOwnerID(t *testing.T) {
	destination := DestinationURL{value: "https://example.com"}
	createdAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	link, err := RehydrateLink(
		"  abc123  ",
		destination,
		"  owner-1  ",
		Active,
		createdAt,
		updatedAt,
		nil,
		1,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if link.Code() != "abc123" {
		t.Fatalf("expected trimmed code %q, got %q", "abc123", link.Code())
	}

	if link.OwnerID() != "owner-1" {
		t.Fatalf("expected trimmed owner ID %q, got %q", "owner-1", link.OwnerID())
	}
}

func assertLinkStateUnchanged(
	t *testing.T,
	link Link,
	expectedCode string,
	expectedDestination DestinationURL,
	expectedOwnerID string,
	expectedStatus LinkStatus,
	expectedCreatedAt time.Time,
	expectedUpdatedAt time.Time,
	expectedExpiresAt *time.Time,
	expectedVersion uint64,
) {
	t.Helper()

	if link.Code() != expectedCode {
		t.Fatalf("expected code to remain %q, got %q", expectedCode, link.Code())
	}

	if link.Destination() != expectedDestination {
		t.Fatalf(
			"expected destination to remain %q, got %q",
			expectedDestination.String(),
			link.Destination().String(),
		)
	}

	if link.OwnerID() != expectedOwnerID {
		t.Fatalf("expected ownerID to remain %q, got %q", expectedOwnerID, link.OwnerID())
	}

	if link.Status() != expectedStatus {
		t.Fatalf("expected status to remain %v, got %v", expectedStatus, link.Status())
	}

	if !link.CreatedAt().Equal(expectedCreatedAt) {
		t.Fatalf("expected createdAt to remain %v, got %v", expectedCreatedAt, link.CreatedAt())
	}

	if !link.UpdatedAt().Equal(expectedUpdatedAt) {
		t.Fatalf("expected updatedAt to remain %v, got %v", expectedUpdatedAt, link.UpdatedAt())
	}

	actualExpiresAt := link.ExpiresAt()
	if expectedExpiresAt == nil && actualExpiresAt != nil {
		t.Fatalf("expected expiresAt to remain nil, got %v", actualExpiresAt)
	}

	if expectedExpiresAt != nil {
		if actualExpiresAt == nil {
			t.Fatal("expected expiresAt to remain non-nil, got nil")
		}

		if !actualExpiresAt.Equal(*expectedExpiresAt) {
			t.Fatalf("expected expiresAt to remain %v, got %v", *expectedExpiresAt, *actualExpiresAt)
		}
	}

	if link.Version() != expectedVersion {
		t.Fatalf("expected version to remain %d, got %d", expectedVersion, link.Version())
	}
}
