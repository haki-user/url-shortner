package ports

import (
	"testing"
	"time"
)

var _ Clock = (*fakeClock)(nil)

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.now
}

func TestFakeClockImplementsClock(t *testing.T) {
	expected := time.Date(2026, 6, 14, 13, 15, 0, 0, time.UTC)

	clock := &fakeClock{now: expected}

	actual := clock.Now()
	if !actual.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}
