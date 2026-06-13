package domain

type LinkStatus uint8

const (
	Unknown LinkStatus = iota
	Active
	Disabled
	Deleted
)

func (s LinkStatus) IsValid() bool {
	switch s {
	case Active, Disabled, Deleted:
		return true
	default:
		return false
	}
}

func (s LinkStatus) CanRedirect() bool {
	return s == Active
}

func (s LinkStatus) CanTransitionTo(next LinkStatus) bool {
	switch s {
	case Active:
		return next == Disabled || next == Deleted
	case Disabled:
		return next == Active || next == Deleted
	default:
		return false
	}
}

func (s LinkStatus) String() string {
	switch s {
	case Active:
		return "active"
	case Disabled:
		return "disabled"
	case Deleted:
		return "deleted"
	default:
		return "unknown"
	}
}
