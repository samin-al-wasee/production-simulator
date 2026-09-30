package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/learning"
)

func runLearn(args []string) int {
	root := findRepoRoot()
	if root == "" {
		fmt.Fprintln(os.Stderr, "forgelab: run inside the ForgeLab repository (ROADMAP.md and backend/go.mod not found)")
		return exitFailed
	}
	path, err := learning.LoadPath(filepath.Join(root, "learning", "path.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	file := progressFile(root)
	progress, err := learning.LoadProgress(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	if len(args) == 0 {
		args = []string{"status"}
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("learn status", flag.ContinueOnError)
		asJSON := fs.Bool("json", false, "print the status as JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return exitUsage
		}
		st := path.Status(progress)
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(st)
			return exitOK
		}
		printStatus(st)
		return exitOK
	case "next":
		st := path.Status(progress)
		if st.Next == "" {
			fmt.Println("learning path complete")
			return exitOK
		}
		for _, s := range st.Stages {
			for _, e := range s.Exercises {
				if e.ID == st.Next {
					fmt.Printf("%s  %s\n  stage: %s\n  how:   %s\n  done by: %s\n", e.ID, e.Title, s.Title, e.How, describeEvidence(e.Evidence))
				}
			}
		}
		return exitOK
	case "complete":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: forgelab learn complete <exercise-id>")
			return exitUsage
		}
		fresh, err := progress.Complete(path, args[1], "manual", time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		if err := progress.Save(file); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		if fresh {
			fmt.Printf("completed %s\n", args[1])
		} else {
			fmt.Printf("%s was already complete\n", args[1])
		}
		return exitOK
	default:
		fmt.Fprintln(os.Stderr, "usage: forgelab learn [status [-json]|next|complete <exercise-id>]")
		return exitUsage
	}
}

func describeEvidence(e learning.Evidence) string {
	if e.Type == learning.EvidenceGoal {
		return fmt.Sprintf("reaching the Sandbox goal %q (or `forgelab learn complete`)", e.Name)
	}
	return "`forgelab learn complete`"
}

func printStatus(st learning.Status) {
	fmt.Printf("learning path: %d of %d exercises complete\n\n", st.Done, st.Total)
	for _, s := range st.Stages {
		mark := " "
		if s.Complete {
			mark = "x"
		}
		fmt.Printf("[%s] %s (%d/%d)\n", mark, s.Title, s.Done, s.Total)
		for _, e := range s.Exercises {
			box := " "
			extra := ""
			if e.Done {
				box = "x"
				extra = "  (" + e.CompletedBy + ", " + e.CompletedAt.Format("2006-01-02") + ")"
			}
			fmt.Printf("    [%s] %-18s %s%s\n", box, e.ID, e.Title, extra)
		}
	}
	if st.Next != "" {
		fmt.Printf("\nnext: %s (forgelab learn next)\n", st.Next)
	} else {
		fmt.Println("\nthe path is complete")
	}
}
