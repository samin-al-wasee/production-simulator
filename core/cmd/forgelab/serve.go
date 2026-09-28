package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/api"
	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
)

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address")
	repo := fs.String("repo", ".", "repository root (contains scenarios/ and manifests/)")
	cluster := fs.String("cluster", "manifests/cluster.example.yaml", "virtual cluster file, relative to -repo")
	reserve := fs.Float64("reserve", budget.DefaultReserveFraction, "fraction of each resource reserved for the host, in [0, 1)")
	enableRuns := fs.Bool("enable-runs", false, "allow starting chaos experiments through the API (loopback addresses only)")
	origin := fs.String("allow-origin", "", "value for Access-Control-Allow-Origin (unset: no CORS headers)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab serve [-addr host:port] [-repo dir] [-cluster file] [-enable-runs] [-allow-origin origin]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}
	if *enableRuns && !isLoopback(*addr) {
		fmt.Fprintln(os.Stderr, "forgelab: -enable-runs requires a loopback listen address (for example 127.0.0.1:8090)")
		return exitUsage
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: api.NewServer(api.Config{
			RepoRoot: *repo, ClusterFile: *cluster, Reserve: *reserve,
			EnableRuns: *enableRuns, AllowedOrigin: *origin,
		}),
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

	fmt.Printf("forgelab API listening on http://%s (experiment runs %s)\n", *addr, map[bool]string{true: "enabled", false: "disabled"}[*enableRuns])
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	return exitOK
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
