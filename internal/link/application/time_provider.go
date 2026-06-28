package application

import (
	"time"
	"tinyurl/internal/link/ports"
)

type TimeProvider struct {
	clock ports.Clock
}

func NewTimeProvider(clock ports.Clock) TimeProvider {
	return TimeProvider{clock: clock}
}

func (t TimeProvider) CurrentTime() time.Time {
	return t.clock.Now()
}
