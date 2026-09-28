package main

import "testing"

func TestIsLoopback(t *testing.T) {
	yes := []string{"127.0.0.1:8090", "localhost:8090", "[::1]:8090"}
	no := []string{"0.0.0.0:8090", ":8090", "192.168.1.5:8090", "example.com:80", "bad"}
	for _, a := range yes {
		if !isLoopback(a) {
			t.Errorf("%s should be loopback", a)
		}
	}
	for _, a := range no {
		if isLoopback(a) {
			t.Errorf("%s should not be loopback", a)
		}
	}
}

func TestServeRefusesRunsOnPublicAddress(t *testing.T) {
	if code := runServe([]string{"-enable-runs", "-addr", "0.0.0.0:8090"}); code != exitUsage {
		t.Fatalf("code = %d, want %d", code, exitUsage)
	}
}
