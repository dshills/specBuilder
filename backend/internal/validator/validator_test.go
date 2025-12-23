package validator

import (
	"testing"
)

func TestValidator(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	t.Run("valid spec", func(t *testing.T) {
		validSpec := `{
			"product": {
				"name": "Test Product",
				"purpose": "A test product",
				"success_criteria": ["Works correctly"]
			},
			"scope": {
				"in_scope": ["Feature A"],
				"out_of_scope": [],
				"assumptions": []
			},
			"personas": [{
				"name": "User",
				"description": "A test user",
				"goals": ["Use the product"]
			}],
			"requirements": {
				"functional": [{
					"id": "FR-001",
					"title": "Test Feature",
					"description": "A test feature",
					"priority": "must",
					"acceptance_criteria": ["Feature works"]
				}],
				"non_functional": [{
					"id": "NFR-001",
					"title": "Performance",
					"description": "Fast response",
					"metric_or_constraint": "<100ms"
				}]
			},
			"workflows": [{
				"id": "WF-001",
				"name": "Main Flow",
				"actors": ["User"],
				"preconditions": [],
				"steps": [{
					"n": 1,
					"action": "User clicks button",
					"system_response": "System shows result"
				}],
				"postconditions": ["Result is shown"]
			}],
			"data_model": {
				"entities": [{
					"name": "Item",
					"description": "An item",
					"fields": [{
						"name": "id",
						"type": "uuid",
						"required": true
					}]
				}]
			},
			"api": {
				"style": "rest",
				"auth": {
					"scheme": "none",
					"authorization": "No auth required"
				},
				"endpoints": [{
					"id": "EP-001",
					"method": "GET",
					"path": "/items",
					"summary": "List items",
					"request": {},
					"responses": [{"status": 200, "body": {}}]
				}],
				"errors": [{
					"code": "NOT_FOUND",
					"message": "Resource not found",
					"http_status": 404
				}]
			},
			"ui": {
				"screens": [{
					"id": "SCR-001",
					"name": "Home",
					"purpose": "Main screen",
					"states": ["loading", "loaded"],
					"validations": []
				}]
			},
			"non_functionals": {
				"performance": "Fast",
				"reliability": "99.9%",
				"security": "Basic",
				"privacy": "No PII",
				"cost": "Low"
			},
			"acceptance": {
				"definition_of_done": ["Feature complete"],
				"test_cases": [{
					"id": "TC-001",
					"name": "Basic test",
					"steps": ["Open app"],
					"expected": ["App opens"]
				}]
			},
			"plan": {
				"milestones": [{
					"id": "M1",
					"name": "MVP",
					"goals": ["Basic functionality"]
				}],
				"tasks": [{
					"id": "T-001",
					"milestone_id": "M1",
					"title": "Implement feature",
					"description": "Build the thing",
					"depends_on": []
				}]
			},
			"trace": {
				"spec_path_to_sources": {
					"/product": [{
						"question_id": "q1",
						"answer_id": "a1",
						"answer_version": 1
					}]
				}
			}
		}`

		result := v.ValidateSpec([]byte(validSpec))
		if !result.Valid {
			t.Errorf("Expected valid spec, got errors: %v", result.Errors)
		}
	})

	t.Run("invalid spec - missing required field", func(t *testing.T) {
		invalidSpec := `{
			"product": {
				"name": "Test",
				"purpose": "Test"
			}
		}`

		result := v.ValidateSpec([]byte(invalidSpec))
		if result.Valid {
			t.Error("Expected invalid spec, got valid")
		}
		if len(result.Errors) == 0 {
			t.Error("Expected errors, got none")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		result := v.ValidateSpec([]byte(`{invalid json`))
		if result.Valid {
			t.Error("Expected invalid, got valid")
		}
	})
}
