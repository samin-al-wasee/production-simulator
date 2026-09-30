package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/api"
)

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address")
	repo := fs.String("repo", ".", "repository root (contains learning/ and manifests/)")
	origin := fs.String("allow-origin", "", "value for Access-Control-Allow-Origin (unset: no CORS headers)")
	progress := fs.String("progress", "", "learning progress file (default <repo>/.forgelab/progress.json)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab serve [-addr host:port] [-repo dir] [-progress file] [-allow-origin origin]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.NewServer(api.Config{RepoRoot: *repo, ProgressFile: *progress, AllowedOrigin: *origin}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	fmt.Printf("forgelab API listening on http://%s\n", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	return exitOK
}
