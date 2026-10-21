package checkflow

import (
	"testing"
	"time"
)

func TestConfirmAndDeadline(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	respondBy := start.Add(Window)

	status, err := Confirm(Pending, start.Add(9*time.Minute), respondBy)
	if err != nil || status != ConfirmedSafe {
		t.Fatalf("early confirm = %s, %v", status, err)
	}

	status, err = Confirm(Pending, respondBy, respondBy)
	if err == nil || status != Unanswered {
		t.Fatalf("deadline confirm = %s, %v", status, err)
	}

	if got := AfterDeadline(Pending, respondBy.Add(-time.Second), respondBy); got != Pending {
		t.Fatalf("still open, got %s", got)
	}
	if got := AfterDeadline(ConfirmedSafe, respondBy.Add(time.Hour), respondBy); got != ConfirmedSafe {
		t.Fatalf("confirmed stays confirmed, got %s", got)
	}
}

func TestValidReason(t *testing.T) {
	for _, reason := range []string{"not_responding", "unwell", "self_harm", "missing"} {
		if !ValidReason(reason) {
			t.Fatalf("expected %s to be valid", reason)
		}
	}
	if ValidReason("volunteer") {
		t.Fatal("unexpected reason accepted")
	}
}
