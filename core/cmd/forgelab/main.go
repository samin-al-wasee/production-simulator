// Command forgelab is the ForgeLab core CLI. It is the thin headless
// interface to ForgeLab's simulation logic; the dashboard never reimplements
// what lives here.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/samin-al-wasee/production-simulator/core/internal/manifest"
)

const (
	exitOK          = 0
	exitBadManifest = 1
	exitUsage       = 2
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "forgelab: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: forgelab <command> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  validate   validate an application manifest against the JSON Schema")
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	schemaPath := fs.String("schema", manifest.DefaultSchemaPath, "path to the application JSON Schema")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab validate [-schema <path>] <manifest-file>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	path := fs.Arg(0)

	validator, err := manifest.NewValidator(*schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitBadManifest
	}

	issues, err := validator.ValidateFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitBadManifest
	}
	if len(issues) > 0 {
		fmt.Fprintln(os.Stderr, "invalid application manifest:")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
		return exitBadManifest
	}

	fmt.Printf("valid application manifest: %s\n", path)
	return exitOK
}
