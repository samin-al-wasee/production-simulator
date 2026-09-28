package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/samin-al-wasee/production-simulator/core/internal/compliance"
	"github.com/samin-al-wasee/production-simulator/core/internal/secretscan"
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
	case "compliance":
		return securityCompliance(args[1:])
	default:
		securityUsage()
		return exitUsage
	}
}

func securityUsage() {
	fmt.Fprintln(os.Stderr, "usage: forgelab security <scan-secrets|compliance> [flags] ...")
	fmt.Fprintln(os.Stderr, "  scan-secrets [-config file] [dir]        find committed credentials (default dir: .)")
	fmt.Fprintln(os.Stderr, "  compliance [-fail-on low|medium|high] <file|dir|-> ...   check Kubernetes and Compose files")
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

func securityCompliance(args []string) int {
	fset := flag.NewFlagSet("security compliance", flag.ContinueOnError)
	failOn := fset.String("fail-on", "medium", "lowest severity that fails the check")
	if err := fset.Parse(args); err != nil {
		return exitUsage
	}
	min, err := compliance.ParseSeverity(*failOn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitUsage
	}
	if fset.NArg() == 0 {
		securityUsage()
		return exitUsage
	}
	inputs := map[string]string{}
	for _, a := range fset.Args() {
		if err := collectYAML(a, inputs); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
	}
	rep, err := compliance.Evaluate(inputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}

	exempt := 0
	for _, f := range rep.Findings {
		tag := strings.ToUpper(f.Control.Severity.String())
		switch {
		case f.Exempt:
			exempt++
			fmt.Printf("EXEMPT %-8s %s %s: %s (reason: %s)\n", tag, f.Control.ID, f.Object, f.Message, f.ExemptReason)
		default:
			fmt.Printf("%-6s %-8s %s %s: %s\n", "FINDING", tag, f.Control.ID, f.Object, f.Message)
		}
	}
	failures := rep.Failures(min)
	fmt.Printf("\n%d object(s) checked, %d finding(s), %d exempt, %d at or above %s\n", rep.Objects, len(rep.Findings), exempt, len(failures), min)
	if len(failures) > 0 {
		fmt.Println("compliance: FAILED")
		return exitFailed
	}
	fmt.Println("compliance: passed (control references are informational, not a certification)")
	return exitOK
}

// collectYAML adds a file, every YAML file under a directory, or stdin ("-").
func collectYAML(path string, into map[string]string) error {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		into["stdin"] = string(b)
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		into[path] = string(b)
		return nil
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".terraform", ".next":
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext == ".yaml" || ext == ".yml" {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			into[p] = string(b)
		}
		return nil
	})
}
