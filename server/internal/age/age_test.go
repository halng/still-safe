package age

import (
	"testing"
	"time"
)

func TestOver16(t *testing.T) {
	now := time.Date(2026, 10, 21, 15, 4, 0, 0, time.UTC)
	cases := []struct {
		dob  string
		want bool
	}{
		{"2010-10-20", true},
		{"2010-10-21", false},
		{"2010-10-22", false},
		{"2000-01-01", true},
		{"2020-01-01", false},
	}
	for _, tc := range cases {
		dob, err := time.Parse("2006-01-02", tc.dob)
		if err != nil {
			t.Fatal(err)
		}
		if got := Over16(dob, now); got != tc.want {
			t.Errorf("Over16(%s) = %v, want %v", tc.dob, got, tc.want)
		}
	}
}
