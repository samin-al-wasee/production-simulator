package secretscan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func rules(line string) []string {
	var r []string
	for _, f := range ScanLine(line) {
		r = append(r, f.Rule)
	}
	return r
}

func TestScanLineDetects(t *testing.T) {
	cases := map[string]string{
		"-----BEGIN RSA PRIVATE KEY-----":       "private-key",
		"-----BEGIN PRIVATE KEY-----":           "private-key",
		"key = AKIAIOSFODNN7EXAMPLE":            "aws-access-key-id",
		"token: ghp_" + strings.Repeat("a", 36): "github-token",
		"SLACK=xoxb-1234567890-abcdefghij":      "slack-token",
		`password: "s3cr3t-Passw0rd"`:           "credential-assignment",
		"DB_PASSWORD=hunter2hunter2":            "credential-assignment",
		`"api_key": "abcdef1234567890"`:         "credential-assignment",
	}
	for line, want := range cases {
		got := rules(line)
		found := false
		for _, r := range got {
			found = found || r == want
		}
		if !found {
			t.Errorf("%q: got %v, want %s", line, got, want)
		}
	}
}

func TestScanLineIgnoresSafeValues(t *testing.T) {
	safe := []string{
		"password: ${POSTGRES_PASSWORD}",
		"PASSWORD=${POSTGRES_PASSWORD:-forgelab}",
		"password: changeme",
		"password: <your-password>",
		"token: ",
		"POSTGRES_PASSWORD_FILE=/run/secrets/db_password",
		"secret: short",
		"# password is set elsewhere",
		"description: rotate the token regularly",
	}
	for _, line := range safe {
		if got := rules(line); len(got) != 0 {
			t.Errorf("%q flagged as %v", line, got)
		}
	}
}

func TestExcerptNeverContainsTheSecret(t *testing.T) {
	secret := "s3cr3t-Passw0rd"
	f := ScanLine("password=" + secret)[0]
	if strings.Contains(f.Excerpt, secret) || strings.Contains(f.Excerpt, secret[2:]) {
		t.Fatalf("excerpt leaks the secret: %q", f.Excerpt)
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanWalksAppliesAllowlistAndSkips(t *testing.T) {
	root := t.TempDir()
	write(t, root, "app/config.yaml", "db:\n  password: realpassword123\n")
	write(t, root, "docs/example.env", "API_KEY=abcdefgh12345678\n")
	write(t, root, "node_modules/x/leak.txt", "password=neverscanned123\n")
	write(t, root, ".git/config", "token=neverscanned123456\n")
	write(t, root, "keys/id.pem", "-----BEGIN PRIVATE KEY-----\nabc\n")

	all, err := Scan(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Path != "app/config.yaml" || all[0].Line != 2 {
		t.Fatalf("findings = %+v", all)
	}

	cfg := Config{Allow: []Allow{
		{Path: "docs/*.env", Reason: "documented sample"},
		{Path: "keys/**", Rule: "private-key", Reason: "test fixture"},
	}}
	got, _ := Scan(root, cfg)
	if len(got) != 1 || got[0].Path != "app/config.yaml" {
		t.Fatalf("after allowlist: %+v", got)
	}
}

func TestScanSkipsGitIgnoredFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init failed: %v %s", err, out)
	}
	write(t, root, ".gitignore", ".env\n")
	write(t, root, ".env", "GITHUB_CLIENT_SECRET=abcdefgh12345678\n")
	write(t, root, "app.yaml", "password: realpassword123\n")

	got, err := Scan(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "app.yaml" {
		t.Fatalf("git-ignored files must not be scanned; got %+v", got)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	if c, err := LoadConfig(filepath.Join(dir, "missing.yaml")); err != nil || len(c.Allow) != 0 {
		t.Fatalf("missing file must be an empty config: %v %v", c, err)
	}
	good := "apiVersion: forgelab/v1\nkind: SecretScan\nallow:\n  - {path: a/*, reason: why}\n"
	write(t, dir, "good.yaml", good)
	if c, err := LoadConfig(filepath.Join(dir, "good.yaml")); err != nil || len(c.Allow) != 1 {
		t.Fatalf("good: %v %v", c, err)
	}
	for name, doc := range map[string]string{
		"no reason": "apiVersion: forgelab/v1\nkind: SecretScan\nallow:\n  - {path: a/*}\n",
		"wrong":     "apiVersion: v1\nkind: SecretScan\n",
		"unknown":   "apiVersion: forgelab/v1\nkind: SecretScan\nbogus: 1\n",
	} {
		write(t, dir, name+".yaml", doc)
		if _, err := LoadConfig(filepath.Join(dir, name+".yaml")); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
