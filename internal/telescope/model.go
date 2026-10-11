// Package telescope is the bounded context for telescopes (Service B).
package telescope

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Allowed values are part of the business rules of this bounded context.
const (
	StatusAvailable   = "AVAILABLE"
	StatusMaintenance = "MAINTENANCE"
)

var (
	validTypes    = map[string]bool{"REFRACTOR": true, "REFLECTOR": true, "CATADIOPTRIC": true}
	validStatuses = map[string]bool{StatusAvailable: true, StatusMaintenance: true}
)

// Telescope is the domain model, independent of protobuf and SQL.
type Telescope struct {
	ID         string
	Name       string
	ApertureMM int32
	Type       string
	Status     string
}

// Normalize trims whitespace and upper-cases enums so " reflector " is accepted.
func (t *Telescope) Normalize() {
	t.Name = strings.TrimSpace(t.Name)
	t.Type = strings.ToUpper(strings.TrimSpace(t.Type))
	t.Status = strings.ToUpper(strings.TrimSpace(t.Status))
}

// Validate enforces required fields, ranges and allowed values.
// It must run before anything is written to the database.
func (t *Telescope) Validate() error {
	switch {
	case t.Name == "":
		return errors.New("name is required")
	case len([]rune(t.Name)) > 100:
		return errors.New("name must be at most 100 characters")
	case t.ApertureMM < 30 || t.ApertureMM > 1000:
		return fmt.Errorf("aperture_mm must be between 30 and 1000, got %d", t.ApertureMM)
	case !validTypes[t.Type]:
		return errors.New("type must be one of REFRACTOR, REFLECTOR, CATADIOPTRIC")
	case !validStatuses[t.Status]:
		return errors.New("status must be one of AVAILABLE, MAINTENANCE")
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
