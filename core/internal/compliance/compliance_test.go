package compliance

import (
	"strings"
	"testing"
)

const hardened = `
apiVersion: v1
kind: Namespace
metadata:
  name: app
  labels:
    pod-security.kubernetes.io/enforce: restricted
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny, namespace: app}
spec:
  podSelector: {}
  policyTypes: [Ingress]
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: app}
spec:
  template:
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: web
          image: registry.example/web:1.2.3
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: [ALL]}
          resources:
            limits: {memory: 128Mi}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: web, namespace: app}
spec:
  tls: [{secretName: web-tls}]
`

func evaluate(t *testing.T, docs ...string) Report {
	t.Helper()
	in := map[string]string{}
	for i, d := range docs {
		in[string(rune('a'+i))+".yaml"] = d
	}
	rep, err := Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func ids(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Control.ID)
	}
	return out
}

func TestHardenedWorkloadPasses(t *testing.T) {
	rep := evaluate(t, hardened)
	if len(rep.Findings) != 0 {
		t.Fatalf("findings = %v", ids(rep.Findings))
	}
	if rep.Objects < 3 {
		t.Fatalf("objects = %d", rep.Objects)
	}
}

func TestInsecureWorkloadIsFlagged(t *testing.T) {
	bad := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: bad, namespace: open}
spec:
  template:
    spec:
      hostNetwork: true
      volumes:
        - {name: h, hostPath: {path: /var/run}}
      containers:
        - name: c
          image: nginx:latest
          securityContext: {privileged: true}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata: {name: plain, namespace: open}
spec:
  rules: []
`
	rep := evaluate(t, bad)
	got := map[string]bool{}
	for _, f := range rep.Findings {
		got[f.Control.ID] = true
	}
	for _, want := range []string{"K8S-001", "K8S-002", "K8S-003", "K8S-004", "K8S-005", "K8S-006", "K8S-007", "K8S-008", "K8S-009", "K8S-010", "K8S-011", "K8S-012", "K8S-013"} {
		if !got[want] {
			t.Errorf("missing finding %s (got %v)", want, ids(rep.Findings))
		}
	}
	if len(rep.Failures(High)) == 0 {
		t.Fatal("expected high-severity failures")
	}
}

func TestContainerNonRootOverridesPod(t *testing.T) {
	doc := strings.Replace(hardened, "runAsNonRoot: true\n        seccompProfile", "runAsNonRoot: false\n        seccompProfile", 1)
	doc = strings.Replace(doc, "allowPrivilegeEscalation: false", "allowPrivilegeEscalation: false\n            runAsNonRoot: true", 1)
	if rep := evaluate(t, doc); len(rep.Findings) != 0 {
		t.Fatalf("container-level runAsNonRoot must satisfy K8S-002: %v", ids(rep.Findings))
	}
}

func TestExemptionRequiresMatchingID(t *testing.T) {
	exempt := strings.Replace(hardened, "metadata: {name: web, namespace: app}\nspec:\n  template",
		"metadata:\n  name: web\n  namespace: app\n  annotations:\n    forgelab.io/compliance-exempt: \"K8S-006=image writes a pid file\"\nspec:\n  template", 1)
	exempt = strings.Replace(exempt, "readOnlyRootFilesystem: true", "readOnlyRootFilesystem: false", 1)
	rep := evaluate(t, exempt)
	if len(rep.Findings) != 1 || !rep.Findings[0].Exempt || rep.Findings[0].ExemptReason != "image writes a pid file" {
		t.Fatalf("findings = %+v", rep.Findings)
	}
	if len(rep.Failures(Low)) != 0 {
		t.Fatal("exempt findings must not fail")
	}
	// An exemption for another control does not hide this one.
	other := strings.Replace(exempt, "K8S-006=", "K8S-001=", 1)
	if r := evaluate(t, other); r.Findings[0].Exempt {
		t.Fatal("exemption for a different control must not apply")
	}
}

func TestExemptionNeedsReason(t *testing.T) {
	if got := parseExemptions("K8S-006=;K8S-002=why"); len(got) != 1 || got["K8S-002"] != "why" {
		t.Fatalf("parseExemptions = %v", got)
	}
}

func TestKustomizationAndUnknownDocsIgnored(t *testing.T) {
	rep := evaluate(t, "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [a.yaml]\n", "# just a comment\n")
	if len(rep.Findings) != 0 || rep.Objects != 0 {
		t.Fatalf("rep = %+v", rep)
	}
}

func TestCompose(t *testing.T) {
	doc := `
services:
  ok:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-forgelab}
      POSTGRES_PASSWORD_FILE: /run/secrets/db
  bad:
    image: nginx
    privileged: true
    network_mode: host
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - API_TOKEN=abc123
  reset:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: !reset null
      POSTGRES_PASSWORD_FILE: /run/secrets/db
  latest:
    image: redis:latest
  exempted:
    image: grafana/promtail:3.3.2
    volumes: ["/var/run/docker.sock:/var/run/docker.sock:ro"]
    x-forgelab-exempt:
      COMP-002: promtail discovers containers through the socket
`
	rep := evaluate(t, doc)
	byObj := map[string][]string{}
	for _, f := range rep.Findings {
		if !f.Exempt {
			byObj[f.Object] = append(byObj[f.Object], f.Control.ID)
		}
	}
	if _, flagged := byObj[`a.yaml service "ok"`]; flagged {
		t.Fatalf("ok service flagged: %v", byObj)
	}
	if got := strings.Join(byObj[`a.yaml service "bad"`], ","); got != "COMP-001,COMP-002,COMP-003,COMP-004,COMP-005" {
		t.Fatalf("bad service findings = %s", got)
	}
	if got := strings.Join(byObj[`a.yaml service "latest"`], ","); got != "COMP-003" {
		t.Fatalf("latest findings = %s", got)
	}
	exempted := 0
	for _, f := range rep.Findings {
		if f.Exempt && f.Control.ID == "COMP-002" {
			exempted++
		}
	}
	if exempted != 1 {
		t.Fatalf("exempted = %d", exempted)
	}
}

func TestSeverityParsing(t *testing.T) {
	for _, s := range []string{"low", "Medium", "HIGH"} {
		if _, err := ParseSeverity(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	if _, err := ParseSeverity("urgent"); err == nil {
		t.Error("expected error")
	}
	if !(Low < Medium && Medium < High) || High.String() != "high" {
		t.Error("severity order")
	}
}

func TestImageTagOK(t *testing.T) {
	ok := []string{"nginx:1.27", "reg.io:5000/team/app:v1", "app@sha256:abc"}
	bad := []string{"nginx", "nginx:latest", "reg.io:5000/app", "app:"}
	for _, i := range ok {
		if !imageTagOK(i) {
			t.Errorf("%s should be ok", i)
		}
	}
	for _, i := range bad {
		if imageTagOK(i) {
			t.Errorf("%s should not be ok", i)
		}
	}
}
