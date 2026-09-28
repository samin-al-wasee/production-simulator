// Command sample-web is the minimal sample application: a plain
// HTTP server with no external dependencies. Its health endpoint reports
// whether the configured PostgreSQL database is reachable, which lets the
// local preset health-check the proxy -> app -> database chain as a unit.
package main

import (
	"embed"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

//go:embed index.html
var indexHTML embed.FS

func main() {
	addr, ok := os.LookupEnv("APP_ADDR")
	if !ok {
		addr = ":8080"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	reg := newRegistry()
	tr := newTracer("sample-web")

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/work", handleWork)
	mux.HandleFunc("/version", handleVersion)
	mux.Handle("/metrics", reg)
	rc := newRedisClient()
	mux.HandleFunc("/api/cache", rc.handleCache)
	mux.HandleFunc("/api/limited", rc.handleLimited)

	logger.Info("sample-web listening", "addr", addr)
	if err := http.ListenAndServe(addr, instrument(mux, reg, logger, tr)); err != nil {
		log.Fatal(err)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := indexHTML.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(page)
}

// handleVersion reports the release identity (APP_VERSION) so rolling,
// canary, and blue/green drills can see which version served a request.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "dev"
	}
	host, _ := os.Hostname()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"version": version, "pod": host})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	dbUp := databaseReachable(time.Second)
	if os.Getenv("APP_UNHEALTHY") == "1" {
		dbUp = false
	}
	w.Header().Set("Content-Type", "application/json")
	if !dbUp {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]any{
		"status": map[bool]string{true: "ok", false: "degraded"}[dbUp],
		"db":     map[bool]string{true: "reachable", false: "unreachable"}[dbUp],
	})
}

// handleWork simulates a unit of work with tunable latency and failure so
// dashboards, alerts, and reliability scenarios have something to observe.
// Query: delay_ms (default 0, max 10000) sleeps, cpu_ms (default 0, max 2000)
// burns CPU, fail=1 returns 500.
func handleWork(w http.ResponseWriter, r *http.Request) {
	delay, _ := strconv.Atoi(r.URL.Query().Get("delay_ms"))
	if delay < 0 {
		delay = 0
	}
	if delay > 10000 {
		delay = 10000
	}
	time.Sleep(time.Duration(delay) * time.Millisecond)
	cpu, _ := strconv.Atoi(r.URL.Query().Get("cpu_ms"))
	if cpu > 2000 {
		cpu = 2000
	}
	for end := time.Now().Add(time.Duration(cpu) * time.Millisecond); time.Now().Before(end); {
	}
	if r.URL.Query().Get("fail") == "1" {
		http.Error(w, "simulated failure", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "delay_ms": delay, "cpu_ms": cpu})
}

func databaseReachable(timeout time.Duration) bool {
	host, ok := os.LookupEnv("DB_HOST")
	if !ok {
		return false
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
