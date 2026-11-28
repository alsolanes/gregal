package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gregal/internal/google"
	"gregal/internal/mcp"
)

// Un compte de Google es connecta amb l'OAuth propi del servidor, queda
// connectat amb el correu com a nom, els vents en llegeixen el calendari amb
// l'eina connector i no hi poden actuar fins que la persona ho permet.
func TestGoogleAccountConnectsReadsAndNeedsPermissionToAct(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = io.WriteString(w, `{"access_token":"ACCESS-SECRET","refresh_token":"REFRESH-SECRET","expires_in":3600}`)
		case "/userinfo":
			_, _ = io.WriteString(w, `{"email":"anna@example.com"}`)
		case "/calendar/v3/calendars/primary/events":
			_, _ = io.WriteString(w, `{"items":[{"summary":"Dentista","start":{"date":"2026-12-03"},"end":{"date":"2026-12-04"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.Close)
	t.Cleanup(google.UseEndpoints(fake.URL))
	s := hubTestServer(t)

	// Sense client OAuth: el catàleg ho diu i connectar explica què falta.
	if code, out := callJSON(t, s, (*Server).handleConnexioConnect, "POST", "/api/connexions/connect", `{"service":"google"}`); code != http.StatusBadGateway || out["code"] != "needs_client_id" {
		t.Fatalf("sense client: %d %v", code, out)
	}
	s.cfg.Google.ClientID, s.cfg.Google.ClientSecret = "id.apps.googleusercontent.com", "shh"

	code, out := callJSON(t, s, (*Server).handleConnexioConnect, "POST", "/api/connexions/connect", `{"service":"google"}`)
	if code != 200 {
		t.Fatalf("connect: %d %v", code, out)
	}
	auth, _ := url.Parse(out["authUrl"].(string))
	if !strings.HasPrefix(out["authUrl"].(string), fake.URL+"/auth") || auth.Query().Get("redirect_uri") != "http://gregal.test"+mcp.OAuthCallbackPath {
		t.Fatalf("consentiment: %s", out["authUrl"])
	}
	rec := httptest.NewRecorder()
	s.handleMCPOAuthCallback(rec, httptest.NewRequest("GET", "http://gregal.test"+mcp.OAuthCallbackPath+"?state="+url.QueryEscape(auth.Query().Get("state"))+"&code=CODE-SECRET", nil))
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "CODE-SECRET") {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	list := listConnexions(t, s)
	if len(list) != 1 || list[0]["service"] != "google" || list[0]["status"] != "connected" || list[0]["accountLabel"] != "anna@example.com" {
		t.Fatalf("connexió: %v", list)
	}
	id := list[0]["id"].(string)

	got, err := s.runConnectorTool(context.Background(), `{"action":"read","connection":"`+id+`","tool":"calendar_events","arguments":{"from":"2026-12-01"}}`)
	if err != nil || !strings.Contains(got, "Dentista") || !strings.Contains(got, "UNTRUSTED") {
		t.Fatalf("lectura: %q %v", got, err)
	}
	if _, err := s.runConnectorTool(context.Background(), `{"action":"act","connection":"`+id+`","tool":"gmail_draft","arguments":{"to":"x@example.com"}}`); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("sense permís d'actuar: %v", err)
	}
	if _, err := s.runConnectorTool(context.Background(), `{"action":"read","connection":"`+id+`","tool":"calendar_create","arguments":{}}`); err == nil {
		t.Fatal("crear no és lectura")
	}
}
