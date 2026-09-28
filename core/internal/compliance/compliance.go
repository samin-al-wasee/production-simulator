// Package compliance evaluates Kubernetes manifests and Docker Compose files
// against a fixed set of hardening controls. Each control has an ID, a
// severity, and informational mappings to public frameworks; the mappings are
// guidance for readers, not a certification. Evaluation is pure: it reads
// YAML text and returns findings.
package compliance

import (
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Severity orders findings.
type Severity int

// Severities from least to most severe.
const (
	Low Severity = iota + 1
	Medium
	High
)

func (s Severity) String() string {
	switch s {
	case Low:
		return "low"
	case Medium:
		return "medium"
	case High:
		return "high"
	}
	return "unknown"
}

// ParseSeverity converts a name to a Severity.
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(s) {
	case "low":
		return Low, nil
	case "medium":
		return Medium, nil
	case "high":
		return High, nil
	}
	return 0, fmt.Errorf("unknown severity %q (want low, medium, or high)", s)
}

// Control describes one check.
type Control struct {
	ID       string
	Title    string
	Severity Severity
	// Refs are informational framework references.
	Refs string
}

// Controls is the catalog, keyed by ID.
var Controls = map[string]Control{
	"K8S-001":  {"K8S-001", "Containers must not be privileged", High, "CIS Kubernetes 5.2.2; NIST 800-53 AC-6"},
	"K8S-002":  {"K8S-002", "Containers must run as non-root", High, "CIS Kubernetes 5.2.6; NIST 800-53 AC-6"},
	"K8S-003":  {"K8S-003", "Privilege escalation must be disabled", Medium, "CIS Kubernetes 5.2.5"},
	"K8S-004":  {"K8S-004", "Containers must drop all capabilities", Medium, "CIS Kubernetes 5.2.7-5.2.9"},
	"K8S-005":  {"K8S-005", "Containers must set a memory limit", Medium, "NIST 800-53 SC-6"},
	"K8S-006":  {"K8S-006", "Root filesystem should be read-only", Low, "CIS Docker 5.12"},
	"K8S-007":  {"K8S-007", "Images must use a pinned, non-latest tag", Medium, "CIS Kubernetes 5.5.1; NIST 800-53 CM-2"},
	"K8S-008":  {"K8S-008", "Pods must not use host namespaces or hostPath volumes", High, "CIS Kubernetes 5.2.3-5.2.4"},
	"K8S-009":  {"K8S-009", "Pods must use a seccomp profile", Medium, "CIS Kubernetes 5.7.2"},
	"K8S-010":  {"K8S-010", "Namespaces must enforce a Pod Security Standard", Medium, "CIS Kubernetes 5.2; Pod Security Admission"},
	"K8S-011":  {"K8S-011", "Namespaces with workloads need a default-deny NetworkPolicy", High, "CIS Kubernetes 5.3.2; NIST 800-53 SC-7"},
	"K8S-012":  {"K8S-012", "Service account tokens must not be automounted", Medium, "CIS Kubernetes 5.1.6"},
	"K8S-013":  {"K8S-013", "Ingress must terminate TLS", High, "NIST 800-53 SC-8"},
	"COMP-001": {"COMP-001", "Compose services must not be privileged", High, "CIS Docker 5.4"},
	"COMP-002": {"COMP-002", "Compose services must not mount the Docker socket", High, "CIS Docker 5.31"},
	"COMP-003": {"COMP-003", "Compose images must use a pinned, non-latest tag", Medium, "CIS Docker 4.7"},
	"COMP-004": {"COMP-004", "Compose services must not use the host network", High, "CIS Docker 5.9"},
	"COMP-005": {"COMP-005", "Credentials must not be literal environment values", High, "NIST 800-53 IA-5"},
}

// Finding is one control result on one object.
type Finding struct {
	Control      Control
	Object       string
	Message      string
	Exempt       bool
	ExemptReason string
}

// Report is the outcome of evaluating a set of documents.
type Report struct {
	Findings []Finding
	// Objects is the number of objects evaluated.
	Objects int
}

// Failures returns non-exempt findings at or above min.
func (r Report) Failures(min Severity) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if !f.Exempt && f.Control.Severity >= min {
			out = append(out, f)
		}
	}
	return out
}

func (r *Report) add(id, object, format string, a ...any) {
	r.Findings = append(r.Findings, Finding{Control: Controls[id], Object: object, Message: fmt.Sprintf(format, a...)})
}

// Evaluate checks documents. Each input is the text of a file that may hold
// several YAML documents (Kubernetes) or one Compose project; name is used
// only in messages.
func Evaluate(inputs map[string]string) (Report, error) {
	var rep Report
	var k8s []k8sObject
	names := make([]string, 0, len(inputs))
	for n := range inputs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, doc := range splitDocs(inputs[name]) {
			var m map[string]any
			if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
				return rep, fmt.Errorf("%s: %w", name, err)
			}
			if len(m) == 0 {
				continue
			}
			if _, ok := m["services"]; ok && m["kind"] == nil {
				rep.Objects++
				checkCompose(&rep, name, m)
				continue
			}
			if o, ok := asK8s(m); ok {
				k8s = append(k8s, o)
			}
		}
	}
	checkKubernetes(&rep, k8s)
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		if rep.Findings[i].Object != rep.Findings[j].Object {
			return rep.Findings[i].Object < rep.Findings[j].Object
		}
		return rep.Findings[i].Control.ID < rep.Findings[j].Control.ID
	})
	return rep, nil
}

func splitDocs(s string) []string {
	var docs []string
	var cur []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimRight(line, " \t\r") == "---" {
			docs = append(docs, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return append(docs, strings.Join(cur, "\n"))
}

// parseExemptions reads "ID=reason;ID2=reason" into a map.
func parseExemptions(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		id, reason, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.TrimSpace(reason) != "" {
			out[strings.TrimSpace(id)] = strings.TrimSpace(reason)
		}
	}
	return out
}

func applyExemptions(rep *Report, from int, ex map[string]string) {
	for i := from; i < len(rep.Findings); i++ {
		if reason, ok := ex[rep.Findings[i].Control.ID]; ok {
			rep.Findings[i].Exempt = true
			rep.Findings[i].ExemptReason = reason
		}
	}
}

// --- generic YAML helpers ---

func str(v any) string {
	s, _ := v.(string)
	return s
}

func mp(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func get(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		cm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = cm[k]
	}
	return cur
}

func isTrue(v any) bool {
	b, _ := v.(bool)
	return b
}

// imageTagOK reports whether an image reference is pinned to a tag other
// than latest, or to a digest.
func imageTagOK(image string) bool {
	if strings.Contains(image, "@sha256:") {
		return true
	}
	name := image
	if i := strings.LastIndex(image, "/"); i >= 0 {
		name = image[i+1:]
	}
	_, tag, ok := strings.Cut(name, ":")
	return ok && tag != "" && tag != "latest"
}
