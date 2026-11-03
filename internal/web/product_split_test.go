package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/jobs"
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

func TestJocsIsItsOwnProduct(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	w := httptest.NewRecorder()
	h.def.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jocs/", nil))
	if !strings.Contains(w.Body.String(), `<html lang="ca" data-product="jocs">`) || !strings.Contains(w.Body.String(), "Gregal Jocs</title>") {
		t.Fatal("/jocs/ must be Gregal Jocs")
	}
}

// Automatitzacions per compte: pausar, reprendre, historial i executar només
// rutines dels vents propis; les d'un altre compte no existeixen.
func TestWindRoutineActionsArePerAccount(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	s := h.def
	j, err := s.jobs.Save(jobs.Job{ID: "rutina-x-1", Name: "Concerts", Kind: "daily", Time: "09:00", Enabled: true, Wind: "xaloc", Prompt: "mira concerts"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.jobs.Delete(j.ID); _ = s.jobs.Delete("rutina-anna-1"); _ = s.jobs.Delete("admin-job") })
	_, _ = s.jobs.Save(jobs.Job{ID: "rutina-anna-1", Name: "D'una altra", Kind: "daily", Time: "09:00", Enabled: true, Wind: "xaloc", WindOwner: "anna"})
	_, _ = s.jobs.Save(jobs.Job{ID: "admin-job", Name: "Sense vent", Kind: "daily", Time: "09:00", Enabled: true})
	call := func(handler func(http.ResponseWriter, *http.Request), method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	if w := call(s.handleWindsRutinaToggle, "POST", "/api/winds/rutines/toggle", `{"id":"rutina-x-1","enabled":false}`); w.Code != 200 {
		t.Fatalf("pause: %d %s", w.Code, w.Body.String())
	}
	if got, _ := s.ownWindJob("rutina-x-1"); got.Enabled {
		t.Fatal("must be paused")
	}
	if w := call(s.handleWindsRutinaToggle, "POST", "/api/winds/rutines/toggle", `{"id":"rutina-x-1","enabled":true}`); w.Code != 200 {
		t.Fatalf("resume: %d", w.Code)
	}
	if got, _ := s.ownWindJob("rutina-x-1"); !got.Enabled || jobs.Due(got, time.Now()) {
		t.Fatal("resumed, but must not fire for the skipped slot")
	}
	if w := call(s.handleWindsRutinaRuns, "GET", "/api/winds/rutines/runs?id=rutina-x-1", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"runs":[]`) {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	for _, id := range []string{"rutina-anna-1", "admin-job"} {
		if w := call(s.handleWindsRutinaRun, "POST", "/api/winds/rutines/run", `{"id":"`+id+`"}`); w.Code != 404 {
			t.Fatalf("run %s must be 404: %d", id, w.Code)
		}
		if w := call(s.handleWindsRutinaToggle, "POST", "/api/winds/rutines/toggle", `{"id":"`+id+`","enabled":false}`); w.Code != 404 {
			t.Fatalf("toggle %s must be 404: %d", id, w.Code)
		}
	}
}
