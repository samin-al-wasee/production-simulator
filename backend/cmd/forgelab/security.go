package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samin-al-wasee/production-simulator/backend/internal/secretscan"
)

const defaultSecretScanConfig = "security/secretscan.yaml"

func runSecurity(args []string) int {
	if len(args) < 1 {
		securityUsage()
		return exitUsage
	}
	switch args[0] {
	case "scan-secrets":
		return securityScanSecrets(args[1:])
	default:
		securityUsage()
		return exitUsage
	}
}

func securityUsage() {
	fmt.Fprintln(os.Stderr, "usage: forgelab security scan-secrets [-config file] [dir]")
	fmt.Fprintln(os.Stderr, "  find committed credentials (default dir: .)")
}

func securityScanSecrets(args []string) int {
	fs := flag.NewFlagSet("security scan-secrets", flag.ContinueOnError)
	cfgPath := fs.String("config", defaultSecretScanConfig, "allowlist file (missing file means no allowlist)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	root := "."
	if fs.NArg() > 1 {
		securityUsage()
		return exitUsage
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	cfgFile := *cfgPath
	if !filepath.IsAbs(cfgFile) && cfgFile == defaultSecretScanConfig {
		cfgFile = filepath.Join(root, cfgFile)
	}
	cfg, err := secretscan.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	findings, err := secretscan.Scan(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	for _, f := range findings {
		fmt.Printf("%s:%d [%s] %s\n", f.Path, f.Line, f.Rule, f.Excerpt)
	}
	if len(findings) > 0 {
		fmt.Printf("\n%d suspected secret(s). Remove them, or allowlist a known local-only default with a reason in %s.\n", len(findings), cfgFile)
		return exitFailed
	}
	fmt.Println("no secrets found")
	return exitOK
}
