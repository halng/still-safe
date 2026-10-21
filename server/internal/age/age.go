package age

import "time"

// Over16 reports whether dob is more than 16 years before now.
// A birthday that lands on today's date sixteen years ago is exactly 16, so it is refused.
func Over16(dob, now time.Time) bool {
	dob = time.Date(dob.Year(), dob.Month(), dob.Day(), 0, 0, 0, 0, time.UTC)
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return dob.Before(now.AddDate(-16, 0, 0))
}
