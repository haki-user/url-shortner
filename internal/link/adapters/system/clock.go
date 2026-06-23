package system

import (
	"time"

	"tinyurl/internal/link/ports"
)

var _ ports.Clock = SystemClock{}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
