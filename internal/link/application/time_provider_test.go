package application

import (
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

func TestTimeProviderCurrentTimeReturnsInjectedClockTime(t *testing.T) {
	fixedTime := time.Date(2026, 6, 15, 13, 15, 0, 0, time.UTC)

	provider := NewTimeProvider(fakeClock{now: fixedTime})

	actual := provider.CurrentTime()
	if !actual.Equal(fixedTime) {
		t.Fatalf("expected %v, got %v", fixedTime, actual)
	}
}
