package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/api"
	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address")
	repo := fs.String("repo", ".", "repository root (contains learning/ and manifests/)")
	origin := fs.String("allow-origin", "", "value for Access-Control-Allow-Origin (unset: no CORS headers)")
	progress := fs.String("progress", "", "learning progress file (default <repo>/.forgelab/progress.json)")
	database := fs.String("database", "", "Postgres URL for the application store (default $DATABASE_URL; unset runs without one)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab serve [-addr host:port] [-repo dir] [-progress file] [-allow-origin origin] [-database url]")
		fmt.Fprintln(fs.Output(), "environment: PORT listens on :PORT unless -addr is set; FORGELAB_ALLOW_ORIGIN sets -allow-origin; DATABASE_URL sets -database")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}
	// A host such as Render gives the port and the dashboard's origin in
	// the environment; flags still win.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if port := os.Getenv("PORT"); port != "" && !set["addr"] {
		*addr = ":" + port
	}
	if o := os.Getenv("FORGELAB_ALLOW_ORIGIN"); o != "" && !set["allow-origin"] {
		*origin = o
	}
	if u := os.Getenv("DATABASE_URL"); u != "" && !set["database"] {
		*database = u
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := api.Config{RepoRoot: *repo, ProgressFile: *progress, AllowedOrigin: *origin}
	if *database != "" {
		st, err := store.Open(ctx, *database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		defer st.Close()
		if err := st.Migrate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		cfg.Store = st
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.NewServer(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}
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
