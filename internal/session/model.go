// Package session is the bounded context for observation sessions (Service A).
// A session books one telescope on one date. It stores only the telescope's
// id - never a copy of its data and never a foreign key into another database.
package session

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusBooked    = "BOOKED"
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"

	dateLayout = "2006-01-02"
)

var validStatuses = map[string]bool{StatusBooked: true, StatusCompleted: true, StatusCancelled: true}

// Session is the domain model.
type Session struct {
	ID          string
	TelescopeID string
	Observer    string
	Date        string // YYYY-MM-DD
	Target      string
	Status      string
}

// Normalize trims whitespace and upper-cases the status.
func (s *Session) Normalize() {
	s.TelescopeID = strings.TrimSpace(s.TelescopeID)
	s.Observer = strings.TrimSpace(s.Observer)
	s.Date = strings.TrimSpace(s.Date)
	s.Target = strings.TrimSpace(s.Target)
	s.Status = strings.ToUpper(strings.TrimSpace(s.Status))
}

// Validate enforces required fields and formats. When requireFutureDate is set
// (creating a booking) the date must not be in the past. `now` is injected so
// the rule can be unit-tested.
func (s *Session) Validate(requireFutureDate bool, now time.Time) error {
	if _, err := uuid.Parse(s.TelescopeID); err != nil {
		return errors.New("telescope_id must be a valid UUID")
	}
	if s.Observer == "" {
		return errors.New("observer is required")
	}
	if len([]rune(s.Observer)) > 100 {
		return errors.New("observer must be at most 100 characters")
	}
	if len([]rune(s.Target)) > 100 {
		return errors.New("target must be at most 100 characters")
	}
	day, err := time.Parse(dateLayout, s.Date)
	if err != nil {
		return errors.New("date is required and must be a real date in YYYY-MM-DD format")
	}
	if requireFutureDate {
		n := now.UTC()
		today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
		if day.Before(today) {
			return errors.New("date cannot be in the past")
		}
	}
	if !validStatuses[s.Status] {
		return errors.New("status must be one of BOOKED, COMPLETED, CANCELLED")
	}
	return nil
}

// ValidateID checks that an identifier is a well-formed UUID.
func ValidateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return errors.New("id must be a valid UUID")
	}
	return nil
}
