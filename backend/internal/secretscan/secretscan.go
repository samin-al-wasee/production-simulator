// Package secretscan finds credentials committed to files. It is a
// defensive, pattern-based scanner: it never prints a secret in full and
// supports an explicit, documented allowlist for known local-only defaults.
package secretscan

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Finding is one suspected secret.
type Finding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Excerpt string `json:"excerpt"`
}

type pattern struct {
	rule string
	re   *regexp.Regexp
	// group is the capture group holding the secret value (0 = whole match).
	group int
}

var patterns = []pattern{
	{"private-key", regexp.MustCompile(`-----BEGIN (?:[A-Z]+ )?PRIVATE KEY-----`), 0},
	{"aws-access-key-id", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), 0},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`), 0},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), 0},
	{"credential-assignment", regexp.MustCompile(`(?i)(?:password|passwd|secret|token|api[_-]?key|access[_-]?key)["']?\s*[:=]\s*["']?([^\s"'$\{\}<>#\-][^\s"'$\{\}<>#]{7,})`), 1},
}

// placeholders are values that are obviously not real secrets.
var placeholders = regexp.MustCompile(`(?i)^(?:changeme|change-me|example|placeholder|your[-_].*|xxx+|\*+|null|none|true|false|redacted|dummy|todo)$`)

// Allow is one allowlist entry: findings in matching paths are ignored, and
// the reason must say why.
type Allow struct {
	Path   string `json:"path"`
	Rule   string `json:"rule,omitempty"`
	Reason string `json:"reason"`
}

// Config is the scanner configuration (kind: SecretScan).
type Config struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Allow      []Allow  `json:"allow"`
	SkipDirs   []string `json:"skipDirs"`
}

// DefaultSkipDirs are never scanned.
var DefaultSkipDirs = []string{".git", "node_modules", ".next", ".terraform", "bin", "vendor", "backups"}

// LoadConfig reads an allowlist file. A missing file is not an error: the
// zero Config scans everything.
func LoadConfig(p string) (Config, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := yaml.UnmarshalStrict(raw, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", p, err)
	}
	if c.APIVersion != "forgelab/v1" || c.Kind != "SecretScan" {
		return Config{}, fmt.Errorf("%s: want apiVersion forgelab/v1 and kind SecretScan", p)
	}
	for i, a := range c.Allow {
		if a.Path == "" || strings.TrimSpace(a.Reason) == "" {
			return Config{}, fmt.Errorf("%s: allow entry %d needs a path and a reason", p, i)
		}
	}
	return c, nil
}

// ScanLine returns the rules that match a single line, with the redacted
// excerpt for each.
func ScanLine(line string) []Finding {
	var out []Finding
	for _, p := range patterns {
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		val := m[p.group]
		if p.rule == "credential-assignment" && placeholders.MatchString(val) {
			continue
		}
		out = append(out, Finding{Rule: p.rule, Excerpt: redact(val)})
	}
	return out
}

func redact(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + strings.Repeat("*", 6) + fmt.Sprintf(" (%d chars)", len(v))
}

func (c Config) allowed(rel, rule string) bool {
	for _, a := range c.Allow {
		if a.Rule != "" && a.Rule != rule {
			continue
		}
		if ok, _ := path.Match(a.Path, rel); ok {
			return true
		}
		if strings.HasSuffix(a.Path, "/**") && strings.HasPrefix(rel, strings.TrimSuffix(a.Path, "**")) {
			return true
		}
	}
	return false
}

// Scan walks root and returns findings not covered by the allowlist, sorted
// by path and line.
func Scan(root string, cfg Config) ([]Finding, error) {
	skip := map[string]bool{}
	for _, d := range DefaultSkipDirs {
		skip[d] = true
	}
	for _, d := range cfg.SkipDirs {
		skip[d] = true
	}
	var out []Finding
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		n := 0
		for sc.Scan() {
			n++
			line := sc.Text()
			if strings.ContainsRune(line, 0) {
				return nil // binary
			}
			for _, fnd := range ScanLine(line) {
				if cfg.allowed(rel, fnd.Rule) {
					continue
				}
				fnd.Path, fnd.Line = rel, n
				out = append(out, fnd)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}
