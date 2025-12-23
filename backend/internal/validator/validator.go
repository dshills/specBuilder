package validator

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.json
var schemasFS embed.FS

// ValidationError represents a single validation error.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationResult holds the result of schema validation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validator validates JSON documents against schemas.
type Validator struct {
	specSchema *jsonschema.Schema
}

// New creates a new Validator with embedded schemas.
func New() (*Validator, error) {
	// Read the spec schema
	schemaData, err := schemasFS.ReadFile("schemas/ProjectImplementationSpec.schema.json")
	if err != nil {
		return nil, fmt.Errorf("read spec schema: %w", err)
	}

	// Unmarshal the schema into an interface{}
	var schemaDoc interface{}
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	// Parse and compile the schema
	c := jsonschema.NewCompiler()

	// Add the schema to the compiler
	if err := c.AddResource("spec.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	schema, err := c.Compile("spec.json")
	if err != nil {
		return nil, fmt.Errorf("compile spec schema: %w", err)
	}

	return &Validator{specSchema: schema}, nil
}

// ValidateSpec validates a compiled spec JSON against the ProjectImplementationSpec schema.
func (v *Validator) ValidateSpec(specJSON []byte) ValidationResult {
	var doc interface{}
	if err := json.Unmarshal(specJSON, &doc); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Path:    "/",
				Message: fmt.Sprintf("invalid JSON: %v", err),
			}},
		}
	}

	err := v.specSchema.Validate(doc)
	if err == nil {
		return ValidationResult{Valid: true}
	}

	// Convert validation error to our format
	var errors []ValidationError
	if ve, ok := err.(*jsonschema.ValidationError); ok {
		errors = extractErrors(ve)
	} else {
		errors = []ValidationError{{
			Path:    "/",
			Message: err.Error(),
		}}
	}

	return ValidationResult{Valid: false, Errors: errors}
}

func extractErrors(ve *jsonschema.ValidationError) []ValidationError {
	var errors []ValidationError

	// Recursively extract errors from causes
	if len(ve.Causes) > 0 {
		for _, cause := range ve.Causes {
			errors = append(errors, extractErrors(cause)...)
		}
	} else {
		// Leaf error
		path := "/" + strings.Join(ve.InstanceLocation, "/")
		errors = append(errors, ValidationError{
			Path:    path,
			Message: ve.Error(),
		})
	}

	return errors
}
