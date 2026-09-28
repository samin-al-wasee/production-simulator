package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// A minimal RESP client: enough for cache-aside and fixed-window rate
// limiting without a third-party dependency. One short-lived connection per
// command keeps it simple and fail-fast.

var errRedisDisabled = fmt.Errorf("REDIS_ADDR not set")

type redisClient struct {
	addr    string
	timeout time.Duration
}

func newRedisClient() *redisClient {
	return &redisClient{addr: os.Getenv("REDIS_ADDR"), timeout: time.Second}
}

func (c *redisClient) do(args ...string) (any, error) {
	if c.addr == "" {
		return nil, errRedisDisabled
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(c.timeout))

	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return nil, err
	}
	return readReply(bufio.NewReader(conn))
}

func readReply(r *bufio.Reader) (any, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, fmt.Errorf("short reply %q", line)
	}
	body := strings.TrimSuffix(line[1:], "\r\n")
	switch line[0] {
	case '+':
		return body, nil
	case '-':
		return nil, fmt.Errorf("redis: %s", body)
	case ':':
		return strconv.ParseInt(body, 10, 64)
	case '$':
		n, err := strconv.Atoi(body)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := readFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	}
	return nil, fmt.Errorf("unsupported reply %q", line)
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := r.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

var cacheHits, cacheMisses, rateLimited atomic.Uint64

// handleCache is a cache-aside endpoint: GET /api/cache?key=k returns the
// value from Redis or computes it (simulated 50 ms origin cost) and caches it
// for 30 seconds.
func (c *redisClient) handleCache(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	v, err := c.do("GET", "cache:"+key)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if s, ok := v.(string); ok {
		cacheHits.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"key": key, "value": s, "source": "cache"})
		return
	}
	cacheMisses.Add(1)
	time.Sleep(50 * time.Millisecond)
	value := "computed:" + key
	if _, err := c.do("SET", "cache:"+key, value, "EX", "30"); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"key": key, "value": value, "source": "origin"})
}

const (
	rateLimitMax    = 10
	rateLimitWindow = 10
)

// handleLimited allows rateLimitMax requests per rateLimitWindow seconds per
// client using a fixed-window INCR/EXPIRE counter.
func (c *redisClient) handleLimited(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		host = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	key := "ratelimit:" + host
	n, err := c.do("INCR", key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	count, _ := n.(int64)
	if count == 1 {
		c.do("EXPIRE", key, strconv.Itoa(rateLimitWindow))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rateLimitMax))
	if count > rateLimitMax {
		rateLimited.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": "rate limit exceeded"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": count, "limit": rateLimitMax})
}
