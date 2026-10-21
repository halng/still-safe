// Package checkflow is the safety-check state machine.
// A check stays pending until the person confirms or the response window ends.
package checkflow

import (
	"errors"
	"time"
)

const Window = 10 * time.Minute

type Status string

const (
	Pending       Status = "pending"
	ConfirmedSafe Status = "confirmed_safe"
	Unanswered    Status = "unanswered"
)

var ErrNotOpen = errors.New("check is not open")

func ValidReason(reason string) bool {
	switch reason {
	case "not_responding", "unwell", "self_harm", "missing":
		return true
	default:
		return false
	}
}

// AfterDeadline moves a pending check to unanswered once the window has closed.
// The deadline itself counts as closed.
func AfterDeadline(status Status, now, respondBy time.Time) Status {
	if status == Pending && !now.Before(respondBy) {
		return Unanswered
	}
	return status
}

// Confirm accepts a pending check that is still inside the window.
func Confirm(status Status, now, respondBy time.Time) (Status, error) {
	status = AfterDeadline(status, now, respondBy)
	if status != Pending {
		return status, ErrNotOpen
	}
	return ConfirmedSafe, nil
}
