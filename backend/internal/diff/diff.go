package diff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

// Change represents a single change between two specs.
type Change struct {
	Path     string          `json:"path"`
	Type     ChangeType      `json:"type"`
	OldValue json.RawMessage `json:"old_value,omitempty"`
	NewValue json.RawMessage `json:"new_value,omitempty"`
}

// Result contains the diff between two specs.
type Result struct {
	Changes  []Change `json:"changes"`
	Summary  Summary  `json:"summary"`
	BaseID   string   `json:"base_id"`
	TargetID string   `json:"target_id"`
}

// Summary provides aggregate counts.
type Summary struct {
	Added    int `json:"added"`
	Removed  int `json:"removed"`
	Modified int `json:"modified"`
	Total    int `json:"total"`
}

// Specs computes the diff between two JSON specs.
func Specs(baseJSON, targetJSON json.RawMessage, baseID, targetID string) (*Result, error) {
	var base, target interface{}

	if len(baseJSON) == 0 {
		baseJSON = []byte("{}")
	}
	if len(targetJSON) == 0 {
		targetJSON = []byte("{}")
	}

	if err := json.Unmarshal(baseJSON, &base); err != nil {
		return nil, fmt.Errorf("unmarshal base: %w", err)
	}
	if err := json.Unmarshal(targetJSON, &target); err != nil {
		return nil, fmt.Errorf("unmarshal target: %w", err)
	}

	changes := compareValues("", base, target)

	// Sort changes by path
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})

	summary := Summary{Total: len(changes)}
	for _, c := range changes {
		switch c.Type {
		case ChangeAdded:
			summary.Added++
		case ChangeRemoved:
			summary.Removed++
		case ChangeModified:
			summary.Modified++
		}
	}

	return &Result{
		Changes:  changes,
		Summary:  summary,
		BaseID:   baseID,
		TargetID: targetID,
	}, nil
}

func compareValues(path string, base, target interface{}) []Change {
	var changes []Change

	// Handle nil cases
	if base == nil && target == nil {
		return changes
	}
	if base == nil {
		changes = append(changes, Change{
			Path:     path,
			Type:     ChangeAdded,
			NewValue: toJSON(target),
		})
		return changes
	}
	if target == nil {
		changes = append(changes, Change{
			Path:     path,
			Type:     ChangeRemoved,
			OldValue: toJSON(base),
		})
		return changes
	}

	// Type mismatch means replacement
	if reflect.TypeOf(base) != reflect.TypeOf(target) {
		changes = append(changes, Change{
			Path:     path,
			Type:     ChangeModified,
			OldValue: toJSON(base),
			NewValue: toJSON(target),
		})
		return changes
	}

	switch b := base.(type) {
	case map[string]interface{}:
		t := target.(map[string]interface{})
		changes = append(changes, compareMaps(path, b, t)...)

	case []interface{}:
		t := target.([]interface{})
		changes = append(changes, compareArrays(path, b, t)...)

	default:
		// Primitive values
		if !reflect.DeepEqual(base, target) {
			changes = append(changes, Change{
				Path:     path,
				Type:     ChangeModified,
				OldValue: toJSON(base),
				NewValue: toJSON(target),
			})
		}
	}

	return changes
}

func compareMaps(path string, base, target map[string]interface{}) []Change {
	var changes []Change

	// Get all keys
	keys := make(map[string]bool)
	for k := range base {
		keys[k] = true
	}
	for k := range target {
		keys[k] = true
	}

	for k := range keys {
		childPath := joinPath(path, k)
		baseVal, baseExists := base[k]
		targetVal, targetExists := target[k]

		if !baseExists {
			changes = append(changes, Change{
				Path:     childPath,
				Type:     ChangeAdded,
				NewValue: toJSON(targetVal),
			})
		} else if !targetExists {
			changes = append(changes, Change{
				Path:     childPath,
				Type:     ChangeRemoved,
				OldValue: toJSON(baseVal),
			})
		} else {
			changes = append(changes, compareValues(childPath, baseVal, targetVal)...)
		}
	}

	return changes
}

func compareArrays(path string, base, target []interface{}) []Change {
	var changes []Change

	maxLen := len(base)
	if len(target) > maxLen {
		maxLen = len(target)
	}

	for i := 0; i < maxLen; i++ {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		if i >= len(base) {
			changes = append(changes, Change{
				Path:     childPath,
				Type:     ChangeAdded,
				NewValue: toJSON(target[i]),
			})
		} else if i >= len(target) {
			changes = append(changes, Change{
				Path:     childPath,
				Type:     ChangeRemoved,
				OldValue: toJSON(base[i]),
			})
		} else {
			changes = append(changes, compareValues(childPath, base[i], target[i])...)
		}
	}

	return changes
}

func joinPath(base, key string) string {
	if base == "" {
		return "/" + key
	}
	return base + "/" + key
}

func toJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ImpactAnalysis analyzes the impact of changes on spec sections.
type ImpactAnalysis struct {
	AffectedSections []string `json:"affected_sections"`
	HighImpact       []string `json:"high_impact"`
	LowImpact        []string `json:"low_impact"`
}

// AnalyzeImpact determines which sections are affected by changes.
func AnalyzeImpact(result *Result) *ImpactAnalysis {
	sectionCounts := make(map[string]int)
	highImpactSections := map[string]bool{
		"product":      true,
		"architecture": true,
		"api":          true,
		"data_model":   true,
	}

	for _, change := range result.Changes {
		// Extract top-level section from path
		section := extractSection(change.Path)
		if section != "" {
			sectionCounts[section]++
		}
	}

	analysis := &ImpactAnalysis{
		AffectedSections: make([]string, 0, len(sectionCounts)),
	}

	for section := range sectionCounts {
		analysis.AffectedSections = append(analysis.AffectedSections, section)
		if highImpactSections[section] {
			analysis.HighImpact = append(analysis.HighImpact, section)
		} else {
			analysis.LowImpact = append(analysis.LowImpact, section)
		}
	}

	sort.Strings(analysis.AffectedSections)
	sort.Strings(analysis.HighImpact)
	sort.Strings(analysis.LowImpact)

	return analysis
}

func extractSection(path string) string {
	// Path format: /section/subsection/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		// Handle array notation
		section := parts[0]
		if idx := strings.Index(section, "["); idx != -1 {
			section = section[:idx]
		}
		return section
	}
	return ""
}
