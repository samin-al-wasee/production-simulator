// Command forgelab is the ForgeLab core CLI. It is the thin headless
// interface to ForgeLab's simulation logic; the dashboard never reimplements
// what lives here.
package main

import (
	"fmt"
	"io"
	"os"
)

const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "pipeline":
		os.Exit(runPipeline(os.Args[2:]))
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "security":
		os.Exit(runSecurity(os.Args[2:]))
	case "learn":
		os.Exit(runLearn(os.Args[2:]))
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
	fmt.Fprintln(w, "  pipeline   simulate a build, test, and deploy pipeline on a virtual clock")
	fmt.Fprintln(w, "  serve      serve the core over HTTP for the dashboard")
	fmt.Fprintln(w, "  security   scan the repository for committed secrets")
	fmt.Fprintln(w, "  learn      show and update progress through the learning path")
}
