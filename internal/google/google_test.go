package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var testCfg = Config{ClientID: "id.apps.googleusercontent.com", ClientSecret: "shh"}

func TestBeginAsksForOfflineAccessWithPKCE(t *testing.T) {
	if _, _, err := Begin(Config{}, "http://x/cb"); err != ErrNotConfigured {
		t.Fatalf("sense client: %v", err)
	}
	u, p, err := Begin(testCfg, "http://127.0.0.1:8097/api/mcp/oauth/callback")
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.ParseQuery(strings.SplitN(u, "?", 2)[1])
	for k, want := range map[string]string{"client_id": testCfg.ClientID, "access_type": "offline", "prompt": "consent",
		"code_challenge_method": "S256", "state": p.State, "redirect_uri": p.Redirect} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, vull %q", k, q.Get(k), want)
		}
	}
	if !strings.Contains(q.Get("scope"), "gmail.compose") || strings.Contains(q.Get("scope"), "gmail.send") {
		t.Errorf("àmbits: %s (esborranys sí, enviar mai)", q.Get("scope"))
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge") == p.Verifier {
		t.Error("el challenge és el hash del verifier, mai el verifier")
	}
}

// fakeGoogle respon com Google: token, userinfo, calendari, Gmail i esborranys.
func fakeGoogle(t *testing.T, drafts *[]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("refresh_token") == "dead" {
				w.WriteHeader(400)
				_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"at-`+r.Form.Get("grant_type")+`","refresh_token":"rt","expires_in":3600,"scope":"openid https://www.googleapis.com/auth/calendar.events"}`)
		case "/userinfo":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(401)
				return
			}
			_, _ = io.WriteString(w, `{"email":"anna@example.com"}`)
		case "/calendar/v3/calendars/primary/events":
			if r.Method == http.MethodPost {
				_, _ = io.WriteString(w, `{"htmlLink":"https://calendar/x"}`)
				return
			}
			_, _ = io.WriteString(w, `{"items":[{"summary":"Dentista","start":{"date":"2026-12-03"},"end":{"date":"2026-12-04"}}]}`)
		case "/gmail/v1/users/me/drafts":
			var body struct{ Message struct{ Raw string } }
			_ = json.NewDecoder(r.Body).Decode(&body)
			raw, _ := base64.RawURLEncoding.DecodeString(body.Message.Raw)
			*drafts = append(*drafts, string(raw))
			_, _ = io.WriteString(w, `{"id":"d1"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	authURL, tokenURL, userinfoURL, apiBase, gmailBase = srv.URL+"/auth", srv.URL+"/token", srv.URL+"/userinfo", srv.URL, srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestExchangeRefreshAndTools(t *testing.T) {
	var drafts []string
	srv := fakeGoogle(t, &drafts)
	ctx := context.Background()
	tok, err := Exchange(ctx, srv.Client(), testCfg, Pending{Verifier: "v", Redirect: "http://x/cb"}, "code")
	if err != nil || tok.Email != "anna@example.com" || tok.RefreshToken != "rt" {
		t.Fatalf("exchange: %+v %v", tok, err)
	}
	tok.Expiry = time.Now().Add(-time.Hour) // caducat: el client el renova sol
	var saved Token
	c := &Client{HTTP: srv.Client(), Cfg: testCfg, Load: func() (Token, error) { return tok, nil }, Save: func(t Token) error { saved = t; return nil }}
	out, err := c.Run(ctx, "calendar_events", `{"from":"2026-12-01","to":"2026-12-08"}`)
	if err != nil || !strings.Contains(out, "Dentista") {
		t.Fatalf("calendar: %q %v", out, err)
	}
	if saved.AccessToken != "at-refresh_token" {
		t.Errorf("el token renovat es desa: %+v", saved)
	}
	if _, err := c.Run(ctx, "gmail_draft", `{"to":"x@example.com\r\nBcc: y@example.com","subject":"hola","body":"b"}`); err == nil {
		t.Error("una capçalera injectada al destinatari s'ha de rebutjar")
	}
	if _, err := c.Run(ctx, "gmail_draft", `{"to":"x@example.com","subject":"Dijous","body":"Em va bé."}`); err != nil || len(drafts) != 1 || !strings.Contains(drafts[0], "To: x@example.com") {
		t.Fatalf("esborrany: %v %q", err, drafts)
	}
	dead := &Client{HTTP: srv.Client(), Cfg: testCfg, Load: func() (Token, error) { return Token{RefreshToken: "dead"}, nil }}
	revoked := false
	dead.OnRevoked = func(error) { revoked = true }
	if _, err := dead.Whoami(ctx); err != ErrRevoked || !revoked {
		t.Errorf("un refresc rebutjat és ErrRevoked i avisa: %v %v", err, revoked)
	}
}

func TestEveryToolHasAScopeAndOnlyDraftsWrite(t *testing.T) {
	for _, tool := range Tools {
		if tool.Scope == "" || tool.Arguments == "" || tool.Description == "" {
			t.Errorf("%s incompleta", tool.Name)
		}
		if strings.Contains(tool.Name, "send") {
			t.Errorf("%s: Gregal no envia correus", tool.Name)
		}
	}
}
