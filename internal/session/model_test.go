package session

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	base := Session{
		TelescopeID: "3f2b8c1e-5d4a-4f6b-9a1c-0d2e7b8c9a10",
		Observer:    "Asha",
		Date:        "2026-10-20",
		Target:      "Saturn",
		Status:      StatusBooked,
	}

	cases := []struct {
		name       string
		mutate     func(*Session)
		requireNew bool
		wantErr    bool
	}{
		{"valid", func(*Session) {}, true, false},
		{"bad telescope id", func(s *Session) { s.TelescopeID = "abc" }, true, true},
		{"missing observer", func(s *Session) { s.Observer = "" }, true, true},
		{"bad date format", func(s *Session) { s.Date = "20/10/2026" }, true, true},
		{"impossible date", func(s *Session) { s.Date = "2026-02-30" }, true, true},
		{"past date on create", func(s *Session) { s.Date = "2026-10-01" }, true, true},
		{"past date allowed on update", func(s *Session) { s.Date = "2026-10-01" }, false, false},
		{"today is allowed", func(s *Session) { s.Date = "2026-10-11" }, true, false},
		{"bad status", func(s *Session) { s.Status = "DONE" }, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := base
			c.mutate(&s)
			s.Normalize()
			if err := s.Validate(c.requireNew, now); (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}
