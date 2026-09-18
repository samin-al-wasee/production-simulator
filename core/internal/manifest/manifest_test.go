package manifest

import (
	"strings"
	"testing"
)

const testSchemaPath = "../../../manifests/application.schema.yaml"

func mustValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator(testSchemaPath)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestValidateValid(t *testing.T) {
	v := mustValidator(t)

	validDocs := []struct {
		name string
		doc  string
	}{
		{
			name: "minimal",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
`,
		},
		{
			name: "full",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: shop-backend
  description: Store API
  owner: platform
spec:
  shape: spa-plus-api
  runtime:
    image: example/api:2.3
    command: ["run"]
    ports: [8080]
    env: ["DATABASE_URL"]
    replicas: 3
  components:
    databases:
      - name: PostgreSQL
        version: "16"
        config:
          max_connections: 100
    networking:
      - name: Reverse Proxy
        version: "1.0"
  scaling:
    autoscale: true
    minReplicas: 1
    maxReplicas: 5
    targetCpuPercent: 70
`,
		},
	}

	for _, tc := range validDocs {
		t.Run(tc.name, func(t *testing.T) {
			issues, err := v.Validate([]byte(tc.doc))
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if len(issues) != 0 {
				t.Fatalf("expected no issues, got: %v", issues)
			}
		})
	}
}

func TestValidateInvalid(t *testing.T) {
	v := mustValidator(t)

	invalidDocs := []struct {
		name         string
		doc          string
		wantIssueSub string
	}{
		{
			name: "missing apiVersion",
			doc: `
kind: Application
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
`,
			wantIssueSub: "apiVersion",
		},
		{
			name: "bad metadata name",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: Bad_Name
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
`,
			wantIssueSub: "/metadata/name",
		},
		{
			name: "unknown component domain",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
  components:
    extras:
      - name: Something
        version: "1"
`,
			wantIssueSub: "/spec/components",
		},
		{
			name: "port out of range",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
    ports: [70000]
`,
			wantIssueSub: "/spec/runtime/ports",
		},
		{
			name: "unknown shape",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
spec:
  shape: wizard
  runtime:
    image: nginx:1.27
`,
			wantIssueSub: "/spec/shape",
		},
		{
			name: "wrong kind",
			doc: `
apiVersion: forgelab/v1
kind: Deployment
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
`,
			wantIssueSub: "kind",
		},
		{
			name: "extra metadata key",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
  tags: ["x"]
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
`,
			wantIssueSub: "/metadata",
		},
		{
			name: "missing spec",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
`,
			wantIssueSub: "spec",
		},
		{
			name: "non-string env element",
			doc: `
apiVersion: forgelab/v1
kind: Application
metadata:
  name: hello
spec:
  shape: monolith
  runtime:
    image: nginx:1.27
    env: [123]
`,
			wantIssueSub: "/spec/runtime/env",
		},
	}

	for _, tc := range invalidDocs {
		t.Run(tc.name, func(t *testing.T) {
			issues, err := v.Validate([]byte(tc.doc))
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if len(issues) == 0 {
				t.Fatalf("expected validation issues, got none")
			}
			found := false
			for _, issue := range issues {
				if strings.Contains(issue, tc.wantIssueSub) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected an issue containing %q, got: %v", tc.wantIssueSub, issues)
			}
		})
	}
}

func TestValidateParseError(t *testing.T) {
	v := mustValidator(t)
	_, err := v.Validate([]byte("[: this is not: valid"))
	if err == nil {
		t.Fatal("expected a parse error for malformed YAML, got nil")
	}
}
