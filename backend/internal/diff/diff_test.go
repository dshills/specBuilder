package diff

import (
	"encoding/json"
	"testing"
)

func TestSpecs(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		target    string
		wantAdded int
		wantRemov int
		wantMod   int
	}{
		{
			name:      "empty to empty",
			base:      `{}`,
			target:    `{}`,
			wantAdded: 0,
			wantRemov: 0,
			wantMod:   0,
		},
		{
			name:      "add field",
			base:      `{}`,
			target:    `{"foo": "bar"}`,
			wantAdded: 1,
			wantRemov: 0,
			wantMod:   0,
		},
		{
			name:      "remove field",
			base:      `{"foo": "bar"}`,
			target:    `{}`,
			wantAdded: 0,
			wantRemov: 1,
			wantMod:   0,
		},
		{
			name:      "modify field",
			base:      `{"foo": "bar"}`,
			target:    `{"foo": "baz"}`,
			wantAdded: 0,
			wantRemov: 0,
			wantMod:   1,
		},
		{
			name:      "nested changes",
			base:      `{"a": {"b": 1, "c": 2}}`,
			target:    `{"a": {"b": 1, "d": 3}}`,
			wantAdded: 1, // d added
			wantRemov: 1, // c removed
			wantMod:   0,
		},
		{
			name:      "array add",
			base:      `{"arr": [1, 2]}`,
			target:    `{"arr": [1, 2, 3]}`,
			wantAdded: 1,
			wantRemov: 0,
			wantMod:   0,
		},
		{
			name:      "array remove",
			base:      `{"arr": [1, 2, 3]}`,
			target:    `{"arr": [1, 2]}`,
			wantAdded: 0,
			wantRemov: 1,
			wantMod:   0,
		},
		{
			name:      "array modify",
			base:      `{"arr": [1, 2, 3]}`,
			target:    `{"arr": [1, 5, 3]}`,
			wantAdded: 0,
			wantRemov: 0,
			wantMod:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Specs(json.RawMessage(tt.base), json.RawMessage(tt.target), "base", "target")
			if err != nil {
				t.Fatalf("Specs() error = %v", err)
			}

			if result.Summary.Added != tt.wantAdded {
				t.Errorf("Added = %d, want %d", result.Summary.Added, tt.wantAdded)
			}
			if result.Summary.Removed != tt.wantRemov {
				t.Errorf("Removed = %d, want %d", result.Summary.Removed, tt.wantRemov)
			}
			if result.Summary.Modified != tt.wantMod {
				t.Errorf("Modified = %d, want %d", result.Summary.Modified, tt.wantMod)
			}
		})
	}
}

func TestAnalyzeImpact(t *testing.T) {
	base := `{"product": {"name": "A"}, "api": {"version": "1"}, "docs": {"readme": "x"}}`
	target := `{"product": {"name": "B"}, "api": {"version": "2"}, "docs": {"readme": "y"}}`

	result, err := Specs(json.RawMessage(base), json.RawMessage(target), "b", "t")
	if err != nil {
		t.Fatalf("Specs() error = %v", err)
	}

	impact := AnalyzeImpact(result)

	if len(impact.AffectedSections) != 3 {
		t.Errorf("AffectedSections len = %d, want 3", len(impact.AffectedSections))
	}

	if len(impact.HighImpact) != 2 { // product and api are high impact
		t.Errorf("HighImpact len = %d, want 2", len(impact.HighImpact))
	}

	if len(impact.LowImpact) != 1 { // docs is low impact
		t.Errorf("LowImpact len = %d, want 1", len(impact.LowImpact))
	}
}
