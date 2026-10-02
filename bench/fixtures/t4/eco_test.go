package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEco(t *testing.T) {
	mux := nouMux()
	// Cas feliç: torna el text en majúscules.
	req := httptest.NewRequest(http.MethodPost, "/api/eco", strings.NewReader(`{"text":"hola món"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/eco: codi %d, volia 200", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"eco":"HOLA MÓN"}` {
		t.Fatalf("POST /api/eco: cos %q, volia %q", rec.Body.String(), `{"eco":"HOLA MÓN"}`)
	}
	// Mètode incorrecte.
	req = httptest.NewRequest(http.MethodGet, "/api/eco", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/eco: codi %d, volia 405", rec.Code)
	}
	// JSON invàlid.
	req = httptest.NewRequest(http.MethodPost, "/api/eco", strings.NewReader(`{text trencat`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/eco amb JSON trencat: codi %d, volia 400", rec.Code)
	}
}
