package main

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeRedis serves a tiny in-memory subset of RESP for tests.
func fakeRedis(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	store := map[string]string{}
	counters := map[string]int64{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				hdr, _ := r.ReadString('\n')
				n := 0
				for _, ch := range strings.TrimSpace(hdr)[1:] {
					n = n*10 + int(ch-'0')
				}
				args := make([]string, n)
				for i := range args {
					r.ReadString('\n')
					args[i], _ = r.ReadString('\n')
					args[i] = strings.TrimSuffix(args[i], "\r\n")
				}
				switch strings.ToUpper(args[0]) {
				case "GET":
					if v, ok := store[args[1]]; ok {
						conn.Write([]byte("$" + itoa(len(v)) + "\r\n" + v + "\r\n"))
					} else {
						conn.Write([]byte("$-1\r\n"))
					}
				case "SET":
					store[args[1]] = args[2]
					conn.Write([]byte("+OK\r\n"))
				case "INCR":
					counters[args[1]]++
					conn.Write([]byte(":" + itoa(int(counters[args[1]])) + "\r\n"))
				case "EXPIRE":
					conn.Write([]byte(":1\r\n"))
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestCacheAside(t *testing.T) {
	c := &redisClient{addr: fakeRedis(t), timeout: 1e9}
	do := func() string {
		rec := httptest.NewRecorder()
		c.handleCache(rec, httptest.NewRequest("GET", "/api/cache?key=a", nil))
		return rec.Body.String()
	}
	if got := do(); !strings.Contains(got, `"source":"origin"`) {
		t.Fatalf("first call: %s", got)
	}
	if got := do(); !strings.Contains(got, `"source":"cache"`) {
		t.Fatalf("second call: %s", got)
	}
}

func TestCacheRequiresKey(t *testing.T) {
	c := &redisClient{addr: fakeRedis(t), timeout: 1e9}
	rec := httptest.NewRecorder()
	c.handleCache(rec, httptest.NewRequest("GET", "/api/cache", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	c := &redisClient{addr: fakeRedis(t), timeout: 1e9}
	var last int
	for i := 0; i < rateLimitMax+2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/limited", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		c.handleLimited(rec, req)
		last = rec.Code
		if i < rateLimitMax && rec.Code != 200 {
			t.Fatalf("request %d rejected", i+1)
		}
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("last code = %d, want 429", last)
	}
}

func TestRedisDisabled(t *testing.T) {
	c := &redisClient{}
	rec := httptest.NewRecorder()
	c.handleCache(rec, httptest.NewRequest("GET", "/api/cache?key=a", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", rec.Code)
	}
}
