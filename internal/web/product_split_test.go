package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// Gregal Codi (/) i Gregal Vents (/vents/) són la mateixa web amb un altre
// producte; les converses dels vents no surten a la llista del Codi.
func TestCodiAndVentsAreSeparateProducts(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	s := h.def
	get := func(path string) string {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Body.String()
	}
	codi, vents := get("/"), get("/vents/")
	if strings.Contains(codi, `data-product="vents"`) || !strings.Contains(codi, "Gregal - Coding Agent") {
		t.Fatal("/ must be Gregal Codi")
	}
	if !strings.Contains(vents, `<html lang="ca" data-product="vents">`) || !strings.Contains(vents, "<title data-i18n=\"ui.titleVents\">Gregal Vents</title>") || !strings.Contains(vents, "vents.webmanifest") {
		t.Fatal("/vents/ must be Gregal Vents")
	}

	for _, name := range []string{"company-xaloc", "rotllana-r1", "feina-de-codi"} {
		if _, err := session.SaveSession(s.sessDir(), session.Session{Name: name, Title: name, Convo: []llm.Message{{Role: "user", Content: "hola"}}}); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	s.handleSessions(w, httptest.NewRequest(http.MethodGet, "/api/sessions", nil))
	body := w.Body.String()
	if !strings.Contains(body, "feina-de-codi") || strings.Contains(body, "company-xaloc") || strings.Contains(body, "rotllana-r1") {
		t.Fatalf("Codi conversations must not list wind chats: %s", body)
	}
}

// Amb comptes, la clau interna de la sessió porta el hash del nom (72
// caràcters): demanar els events sense ?session= no pot fallar amb
// «sessió invàlida» (passava a tots els comptes del servidor).
func TestEventsWithoutSessionWorkForAccounts(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", t.TempDir()+"/accounts")
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	if w := accountsPost(t, s, `{"name":"carla","password":"12345678"}`); w.Code != 200 {
		t.Fatalf("create: %d", w.Code)
	}
	tok, _, err := s.users.Login("carla", "12345678")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v2/events", "/api/v2/events?session=default"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != 200 || strings.Contains(w.Body.String(), "sessió invàlida") {
			t.Fatalf("%s as an account: %d %s", path, w.Code, w.Body.String())
		}
	}
}
