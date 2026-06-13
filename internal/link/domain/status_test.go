package domain

import "testing"

func TestLinkStatusIsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   LinkStatus
		expected bool
	}{
		{name: "unknown is invalid", status: Unknown, expected: false},
		{name: "active is valid", status: Active, expected: true},
		{name: "disabled is valid", status: Disabled, expected: true},
		{name: "deleted is valid", status: Deleted, expected: true},
		{name: "invalid numeric value is invalid", status: LinkStatus(200), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.status.IsValid()
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestLinkStatusCanRedirect(t *testing.T) {
	tests := []struct {
		name     string
		status   LinkStatus
		expected bool
	}{
		{name: "unknown cannot redirect", status: Unknown, expected: false},
		{name: "active can redirect", status: Active, expected: true},
		{name: "disabled cannot redirect", status: Disabled, expected: false},
		{name: "deleted cannot redirect", status: Deleted, expected: false},
		{name: "invalid numeric value cannot redirect", status: LinkStatus(200), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.status.CanRedirect()
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestLinkStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		status   LinkStatus
		next     LinkStatus
		expected bool
	}{
		{name: "active can transition to disabled", status: Active, next: Disabled, expected: true},
		{name: "active can transition to deleted", status: Active, next: Deleted, expected: true},
		{name: "disabled can transition to active", status: Disabled, next: Active, expected: true},
		{name: "disabled can transition to deleted", status: Disabled, next: Deleted, expected: true},

		{name: "active cannot transition to active", status: Active, next: Active, expected: false},
		{name: "active cannot transition to unknown", status: Active, next: Unknown, expected: false},
		{name: "active cannot transition to invalid numeric value", status: Active, next: LinkStatus(200), expected: false},

		{name: "disabled cannot transition to disabled", status: Disabled, next: Disabled, expected: false},
		{name: "disabled cannot transition to unknown", status: Disabled, next: Unknown, expected: false},
		{name: "disabled cannot transition to invalid numeric value", status: Disabled, next: LinkStatus(200), expected: false},

		{name: "deleted cannot transition to unknown", status: Deleted, next: Unknown, expected: false},
		{name: "deleted cannot transition to active", status: Deleted, next: Active, expected: false},
		{name: "deleted cannot transition to disabled", status: Deleted, next: Disabled, expected: false},
		{name: "deleted cannot transition to deleted", status: Deleted, next: Deleted, expected: false},

		{name: "unknown cannot transition to unknown", status: Unknown, next: Unknown, expected: false},
		{name: "unknown cannot transition to active", status: Unknown, next: Active, expected: false},
		{name: "unknown cannot transition to disabled", status: Unknown, next: Disabled, expected: false},
		{name: "unknown cannot transition to deleted", status: Unknown, next: Deleted, expected: false},

		{name: "invalid numeric value cannot transition to active", status: LinkStatus(200), next: Active, expected: false},
		{name: "invalid numeric value cannot transition to disabled", status: LinkStatus(200), next: Disabled, expected: false},
		{name: "invalid numeric value cannot transition to deleted", status: LinkStatus(200), next: Deleted, expected: false},
		{name: "invalid numeric value cannot transition to Unknown", status: LinkStatus(200), next: Unknown, expected: false},
		{name: "invalid numeric value cannot transition to invalid numeric value", status: LinkStatus(200), next: LinkStatus(201), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.status.CanTransitionTo(tt.next)
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestLinkStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   LinkStatus
		expected string
	}{
		{name: "invalid numeric value", status: LinkStatus(200), expected: "unknown"},
		{name: "unknown string", status: Unknown, expected: "unknown"},
		{name: "active string", status: Active, expected: "active"},
		{name: "disabled string", status: Disabled, expected: "disabled"},
		{name: "deleted string", status: Deleted, expected: "deleted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.status.String()
			if actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
