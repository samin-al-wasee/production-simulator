package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	t.Setenv("APP_VERSION", "v9")
	rec := httptest.NewRecorder()
	handleVersion(rec, httptest.NewRequest("GET", "/version", nil))
	if !strings.Contains(rec.Body.String(), `"version":"v9"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestUnhealthySwitch(t *testing.T) {
	t.Setenv("APP_UNHEALTHY", "1")
	rec := httptest.NewRecorder()
	handleHealth(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 503 {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}
