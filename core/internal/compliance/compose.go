package compliance

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var credentialKey = regexp.MustCompile(`(?i)(password|secret|token|api[_-]?key)`)
var fileSuffix = regexp.MustCompile(`(?i)_FILE$`)

func checkCompose(rep *Report, file string, m map[string]any) {
	services := mp(m["services"])
	names := make([]string, 0, len(services))
	for n := range services {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		svc := mp(services[name])
		obj := fmt.Sprintf("%s service %q", file, name)
		from := len(rep.Findings)

		if isTrue(svc["privileged"]) {
			rep.add("COMP-001", obj, "privileged: true")
		}
		if str(svc["network_mode"]) == "host" {
			rep.add("COMP-004", obj, "network_mode: host")
		}
		if img := str(svc["image"]); img != "" && !imageTagOK(img) {
			rep.add("COMP-003", obj, "image %q is unpinned or uses latest", img)
		}
		for _, v := range list(svc["volumes"]) {
			src := str(v)
			if m := mp(v); m != nil {
				src = str(m["source"])
			}
			if strings.Contains(src, "docker.sock") {
				rep.add("COMP-002", obj, "mounts the Docker socket (%s)", src)
			}
		}
		for key, val := range envPairs(svc["environment"]) {
			if !credentialKey.MatchString(key) || fileSuffix.MatchString(key) {
				continue
			}
			if val != "" && val != "null" && val != "<nil>" && !strings.Contains(val, "${") {
				rep.add("COMP-005", obj, "environment %s holds a literal value; use interpolation or a *_FILE secret", key)
			}
		}
		applyExemptions(rep, from, exemptMap(svc["x-forgelab-exempt"]))
	}
}

func exemptMap(v any) map[string]string {
	out := map[string]string{}
	for k, val := range mp(v) {
		if s := strings.TrimSpace(str(val)); s != "" {
			out[k] = s
		}
	}
	return out
}

// envPairs normalizes Compose environment (map or KEY=VALUE list).
func envPairs(v any) map[string]string {
	out := map[string]string{}
	if m := mp(v); m != nil {
		for k, val := range m {
			out[k] = fmt.Sprint(val)
			if val == nil {
				out[k] = ""
			}
		}
		return out
	}
	for _, e := range list(v) {
		k, val, _ := strings.Cut(str(e), "=")
		out[k] = val
	}
	return out
}
