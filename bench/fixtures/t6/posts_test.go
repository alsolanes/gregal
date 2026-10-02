package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func crida(mux *http.ServeMux, metode, ruta, cos string) *httptest.ResponseRecorder {
	var req *http.Request
	if cos == "" {
		req = httptest.NewRequest(metode, ruta, nil)
	} else {
		req = httptest.NewRequest(metode, ruta, strings.NewReader(cos))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCrearPost(t *testing.T) {
	mux := nouMux()
	rec := crida(mux, http.MethodPost, "/api/posts", `{"titol":"Nou","cos":"Contingut nou"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/posts: codi %d, volia 201", rec.Code)
	}
	cos := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(cos, `"id":3`) || !strings.Contains(cos, `"titol":"Nou"`) {
		t.Fatalf("POST /api/posts: cos %q sense id 3 i títol", cos)
	}
	// El creat es pot llegir.
	rec = crida(mux, http.MethodGet, "/api/posts/3", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Contingut nou") {
		t.Fatalf("GET /api/posts/3: codi %d cos %q", rec.Code, rec.Body.String())
	}
}

func TestCrearPostInvalid(t *testing.T) {
	mux := nouMux()
	for _, cos := range []string{`{"titol":"","cos":"x"}`, `{"titol":"x"}`, `{"titol":"x","cos":""}`} {
		rec := crida(mux, http.MethodPost, "/api/posts", cos)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("POST /api/posts amb %s: codi %d, volia 400", cos, rec.Code)
		}
	}
	rec := crida(mux, http.MethodPost, "/api/posts", `{json trencat`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/posts amb JSON trencat: codi %d, volia 400", rec.Code)
	}
}

func TestLlegirPost(t *testing.T) {
	mux := nouMux()
	rec := crida(mux, http.MethodGet, "/api/posts/1", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Benvinguts") {
		t.Fatalf("GET /api/posts/1: codi %d cos %q", rec.Code, rec.Body.String())
	}
	rec = crida(mux, http.MethodGet, "/api/posts/999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/posts/999: codi %d, volia 404", rec.Code)
	}
}

func TestEsborrarPost(t *testing.T) {
	mux := nouMux()
	rec := crida(mux, http.MethodDelete, "/api/posts/2", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/posts/2: codi %d, volia 204", rec.Code)
	}
	rec = crida(mux, http.MethodGet, "/api/posts/2", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/posts/2 després d'esborrar: codi %d, volia 404", rec.Code)
	}
	rec = crida(mux, http.MethodDelete, "/api/posts/999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE /api/posts/999: codi %d, volia 404", rec.Code)
	}
}
