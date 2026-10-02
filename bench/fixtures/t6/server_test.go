package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLlistaPosts(t *testing.T) {
	mux := nouMux()
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/posts: codi %d, volia 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Benvinguts") {
		t.Fatalf("GET /api/posts: cos %q sense el primer post", rec.Body.String())
	}
}
