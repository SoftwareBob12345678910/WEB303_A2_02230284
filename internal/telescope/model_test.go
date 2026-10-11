package telescope

import "testing"

func TestValidate(t *testing.T) {
	ok := Telescope{Name: "Dobsonian 8", ApertureMM: 203, Type: "REFLECTOR", Status: "AVAILABLE"}

	cases := []struct {
		name    string
		mutate  func(*Telescope)
		wantErr bool
	}{
		{"valid", func(*Telescope) {}, false},
		{"missing name", func(x *Telescope) { x.Name = "" }, true},
		{"aperture too small", func(x *Telescope) { x.ApertureMM = 5 }, true},
		{"aperture too large", func(x *Telescope) { x.ApertureMM = 5000 }, true},
		{"bad type", func(x *Telescope) { x.Type = "BINOCULARS" }, true},
		{"bad status", func(x *Telescope) { x.Status = "BROKEN" }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tel := ok
			c.mutate(&tel)
			tel.Normalize()
			if err := tel.Validate(); (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestNormalizeAcceptsLowercase(t *testing.T) {
	tel := Telescope{Name: "  Refractor 80 ", ApertureMM: 80, Type: " refractor ", Status: "available"}
	tel.Normalize()
	if err := tel.Validate(); err != nil {
		t.Fatalf("expected normalized telescope to be valid: %v", err)
	}
	if tel.Name != "Refractor 80" {
		t.Fatalf("name not trimmed: %q", tel.Name)
	}
}

func TestValidateID(t *testing.T) {
	if err := ValidateID("not-a-uuid"); err == nil {
		t.Fatal("expected error for malformed id")
	}
	if err := ValidateID("3f2b8c1e-5d4a-4f6b-9a1c-0d2e7b8c9a10"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
