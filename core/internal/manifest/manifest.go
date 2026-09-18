// Package manifest loads and validates ForgeLab application manifests
// against manifests/application.schema.yaml.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"sigs.k8s.io/yaml"
)

// DefaultSchemaPath is the repository-relative location of the canonical
// application JSON Schema.
const DefaultSchemaPath = "manifests/application.schema.yaml"

// Validator validates application manifests against a compiled JSON Schema.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator reads, converts, and compiles the JSON Schema found at
// schemaPath. The schema file is YAML (per the repo convention) and must be
// valid JSON Schema once converted.
func NewValidator(schemaPath string) (*Validator, error) {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %q: %w", schemaPath, err)
	}
	schemaJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse schema %q: %w", schemaPath, err)
	}
	compiler := jsonschema.NewCompiler()
	var schemaDoc any
	if err := json.Unmarshal(schemaJSON, &schemaDoc); err != nil {
		return nil, fmt.Errorf("decode schema %q: %w", schemaPath, err)
	}
	if err := compiler.AddResource("application.schema.yaml", schemaDoc); err != nil {
		return nil, fmt.Errorf("load schema %q: %w", schemaPath, err)
	}
	schema, err := compiler.Compile("application.schema.yaml")
	if err != nil {
		return nil, fmt.Errorf("compile schema %q: %w", schemaPath, err)
	}
	return &Validator{schema: schema}, nil
}

// ValidateFile reads the manifest at path and validates it.
// It returns a list of human-readable issues; an empty list means the
// manifest is valid.
func (v *Validator) ValidateFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	return v.Validate(raw)
}

// Validate checks raw application-manifest bytes (YAML or JSON) against the
// schema. It returns a list of human-readable issues; an empty list means the
// document is valid.
func (v *Validator) Validate(raw []byte) ([]string, error) {
	docJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	var doc any
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := v.schema.Validate(doc); err != nil {
		var issues []string
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			for _, cause := range ve.Causes {
				issues = append(issues, fmt.Sprintf("%s: %s", cause.InstanceLocation, cause.Error()))
			}
			if len(ve.Causes) == 0 {
				issues = append(issues, fmt.Sprintf("%s: %s", ve.InstanceLocation, ve.Error()))
			}
		} else {
			return nil, fmt.Errorf("validate manifest: %w", err)
		}
		return issues, nil
	}
	return nil, nil
}
