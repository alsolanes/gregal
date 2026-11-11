package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/connexions"
	"gregal/internal/mcp"
	"gregal/internal/runs"
)

// B5 — connexions. Un proveïdor OAuth + MCP de prova local (cap compte
// real): metadades, registre dinàmic, PKCE, refresc, revocació i dues eines
// (una de lectura, una que actua).

type fakeProvider struct {
	t   *testing.T
	srv *httptest.Server
	mu  sync.Mutex
	// codes: codi → {challenge, email}
	codes map[string][2]string
	// access/refresh vàlids → email
	access  map[string]string
	refresh map[string]string
	revoked []string
	n       int
	// shortLived: els tokens caduquen de seguida (força el refresc).
	shortLived bool
	refreshed  int
	calls      []string // email|eina
}

func newFakeProvider(t *testing.T) *fakeProvider {
	f := &fakeProvider{t: t, codes: map[string][2]string{}, access: map[string]string{}, refresh: map[string]string{}}
	mux := http.NewServeMux()
	meta := func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize", "token_endpoint": f.srv.URL + "/token",
			"registration_endpoint": f.srv.URL + "/register", "revocation_endpoint": f.srv.URL + "/revoke",
			"code_challenge_methods_supported": []string{"S256"},
		})
	}
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"resource": f.srv.URL + "/mcp", "authorization_servers": []string{f.srv.URL}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", meta)
	mux.HandleFunc("/register", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "cid-gregal"})
	})
	mux.HandleFunc("/token", f.token)
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		tok := r.Form.Get("token")
		f.revoked = append(f.revoked, tok)
		delete(f.access, tok)
		delete(f.refresh, tok)
		f.mu.Unlock()
		w.WriteHeader(200)
	})
	mux.HandleFunc("/mcp", f.mcp)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// authorize fa de la persona al consentiment: torna el codi per a email.
func (f *fakeProvider) authorize(authURL, email string) (state, code string) {
	u, err := url.Parse(authURL)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "cid-gregal" {
		f.t.Fatalf("consentiment sense PKCE o client: %s", authURL)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	code = fmt.Sprintf("CODE-SECRET-%d", f.n)
	f.codes[code] = [2]string{q.Get("code_challenge"), email}
	return q.Get("state"), code
}

func jwtFor(email string) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	return enc(map[string]string{"alg": "none"}) + "." + enc(map[string]string{"email": email}) + ".sig"
}

func (f *fakeProvider) issue(w http.ResponseWriter, email string) {
	f.n++
	acc, ref := fmt.Sprintf("ACCESS-SECRET-%d", f.n), fmt.Sprintf("REFRESH-SECRET-%d", f.n)
	f.access[acc], f.refresh[ref] = email, email
	exp := 3600
	if f.shortLived {
		exp = 1 // amb el marge de 30 s, ja caducat
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": acc, "refresh_token": ref, "token_type": "Bearer", "expires_in": exp, "id_token": jwtFor(email)})
}

func (f *fakeProvider) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		c, ok := f.codes[r.Form.Get("code")]
		delete(f.codes, r.Form.Get("code"))
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != c[0] {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}
		f.issue(w, c[1])
	case "refresh_token":
		email, ok := f.refresh[r.Form.Get("refresh_token")]
		if !ok {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}
		delete(f.refresh, r.Form.Get("refresh_token"))
		f.refreshed++
		f.issue(w, email)
	default:
		w.WriteHeader(400)
	}
}

func (f *fakeProvider) mcp(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		w.WriteHeader(204)
		return
	}
	f.mu.Lock()
	email, ok := f.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	f.mu.Unlock()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+f.srv.URL+`/.well-known/oauth-protected-resource/mcp"`)
		http.Error(w, "unauthorized", 401)
		return
	}
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&msg)
	if len(msg.ID) == 0 {
		w.WriteHeader(202)
		return
	}
	var res any
	switch msg.Method {
	case "initialize":
		res = map[string]any{"protocolVersion": mcp.ProtocolVersion, "serverInfo": map[string]any{"name": "notes", "version": "1"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		w.Header().Set("Mcp-Session-Id", "s1")
	case "tools/list":
		res = map[string]any{"tools": []any{
			map[string]any{"name": "list_notes", "description": "List notes", "inputSchema": map[string]any{"type": "object"}, "annotations": map[string]any{"readOnlyHint": true}},
			map[string]any{"name": "create_note", "description": "Create a note", "inputSchema": map[string]any{"type": "object"}},
		}}
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		f.mu.Lock()
		f.calls = append(f.calls, email+"|"+p.Name)
		f.mu.Unlock()
		res = map[string]any{"content": []any{map[string]any{"type": "text", "text": "notes of " + email + " via " + p.Name}}}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": res})
}

// useProvider posa el connector «notes» (remot) al gestor del servidor.
func useNotesConnector(t *testing.T, s *Server, f *fakeProvider) *mcp.Manager {
	t.Helper()
	m := mcp.NewManager(map[string]mcp.Server{"notes": {URL: f.srv.URL + "/mcp"}}, mcp.Options{Store: mcp.NewTokenStore(filepath.Join(t.TempDir(), "mcp.json")), NoRegister: true})
	s.mcp = m
	return m
}

func callJSON(t *testing.T, s *Server, fn func(*Server, http.ResponseWriter, *http.Request), method, path, body string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "http://gregal.test"+path, strings.NewReader(body))
	fn(s, rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// connectAccount fa tot el camí: Connecta → consentiment → callback.
func connectAccount(t *testing.T, s *Server, f *fakeProvider, body, email string) (map[string]any, string) {
	t.Helper()
	code, out := callJSON(t, s, (*Server).handleConnexioConnect, "POST", "/api/connexions/connect", body)
	if code != 200 {
		t.Fatalf("connect: %d %v", code, out)
	}
	authURL, _ := out["authUrl"].(string)
	state, c := f.authorize(authURL, email)
	rec := httptest.NewRecorder()
	s.handleMCPOAuthCallback(rec, httptest.NewRequest("GET", "http://gregal.test"+mcp.OAuthCallbackPath+"?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(c), nil))
	page := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("callback: %d %s", rec.Code, page)
	}
	if strings.Contains(page, c) {
		t.Fatal("el codi no pot sortir a la pàgina")
	}
	return out["connexio"].(map[string]any), page
}

// assertNoSecrets: cap secret en el que surt del servidor ni als fitxers en clar.
func assertNoSecrets(t *testing.T, where string, text string) {
	t.Helper()
	for _, sec := range []string{"ACCESS-SECRET", "REFRESH-SECRET", "CODE-SECRET", "app-pass-123", "private-cal-secret", "verifier"} {
		if strings.Contains(text, sec) {
			t.Fatalf("%s porta un secret (%s):\n%s", where, sec, text)
		}
	}
}

func listConnexions(t *testing.T, s *Server) []map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleConnexions(rec, httptest.NewRequest("GET", "/api/connexions", nil))
	assertNoSecrets(t, "GET /api/connexions", rec.Body.String())
	var out struct {
		Connexions []map[string]any `json:"connexions"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	for _, c := range out.Connexions {
		// La forma del contracte (fixture connexio-caducada.json).
		for _, k := range []string{"id", "owner", "method", "accountLabel", "status", "capabilities", "authRef"} {
			if _, ok := c[k]; !ok {
				t.Fatalf("falta %s: %v", k, c)
			}
		}
		if ref, _ := c["authRef"].(string); !strings.HasPrefix(ref, "auth:") || !containsString(connexioStates, c["status"].(string)) {
			t.Fatalf("authRef o estat: %v", c)
		}
	}
	return out.Connexions
}

func TestConnexioOAuthEndToEndReuseRefreshRevokeAndTwoAccounts(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	f := newFakeProvider(t)
	s := hubTestServer(t)
	useNotesConnector(t, s, f)
	ctx := context.Background()

	conn, page := connectAccount(t, s, f, `{"service":"mcp","provider":"notes","returnTo":"xyz.solanes.gregal://connexions"}`, "anna@example.org")
	id := conn["id"].(string)
	if !strings.Contains(page, "anna@example.org") || !strings.Contains(page, "xyz.solanes.gregal://connexions?id="+id) {
		t.Fatalf("«connectat com…» i tornada a l'app:\n%s", page)
	}
	list := listConnexions(t, s)
	if len(list) != 1 || list[0]["status"] != "connected" || list[0]["accountLabel"] != "anna@example.org" || list[0]["lastCheckedAt"] == nil {
		t.Fatalf("connectat i comprovat: %v", list)
	}
	if caps := fmt.Sprint(list[0]["capabilities"]); caps != "[read send]" || fmt.Sprint(list[0]["effective"]) != "[read]" {
		t.Fatalf("capacitat de l'eina vs autorització: %v", list[0])
	}

	// Una segona i una tercera feina fan servir el compte sense cap login nou.
	for i := 0; i < 2; i++ {
		out, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id+`","tool":"list_notes"}`)
		if err != nil || !strings.Contains(out, "notes of anna@example.org") || !strings.Contains(out, "UNTRUSTED") {
			t.Fatalf("feina %d: %v %s", i+1, err, out)
		}
	}
	// act necessita el permís de la persona, i read no serveix per actuar.
	if _, err := s.runConnectorTool(ctx, `{"action":"act","connection":"`+id+`","tool":"create_note"}`); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("act sense permís: %v", err)
	}
	if _, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id+`","tool":"create_note"}`); err == nil {
		t.Fatal("una eina que actua no passa per read")
	}
	if code, _ := callJSON(t, s, (*Server).handleConnexioUpdate, "POST", "/api/connexions/update", `{"id":"`+id+`","allowed":["read","send"]}`); code != 200 {
		t.Fatal("update")
	}
	if out, err := s.runConnectorTool(ctx, `{"action":"act","connection":"`+id+`","tool":"create_note"}`); err != nil || !strings.Contains(out, "create_note") {
		t.Fatalf("act amb permís: %v %s", err, out)
	}
	if d, _ := (&agent.Policy{}).Decide(agent.ModeCode, "connector", `{"action":"act"}`); d != "ask" {
		t.Fatalf("cada act es confirma: %s", d)
	}

	// Refresc: el token caduca i es renova sol.
	f.mu.Lock()
	f.shortLived = true
	f.mu.Unlock()
	conn2, _ := connectAccount(t, s, f, `{"service":"mcp","provider":"notes","label":"personal"}`, "bernat@example.org")
	id2 := conn2["id"].(string)
	if _, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id2+`","tool":"list_notes"}`); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	refreshed := f.refreshed
	f.mu.Unlock()
	if refreshed == 0 {
		t.Fatal("un token caducat s'ha de refrescar")
	}
	// Dos comptes del mateix connector, distingibles.
	list = listConnexions(t, s)
	if len(list) != 2 || list[0]["id"] == list[1]["id"] || list[1]["accountLabel"] != "personal" {
		t.Fatalf("dos comptes distingibles: %v", list)
	}
	if out := s.connectorList(); !strings.Contains(out, id) || !strings.Contains(out, id2) || !strings.Contains(out, "personal") {
		t.Fatalf("llista per al model:\n%s", out)
	}

	// Desconnectar: revoca al proveïdor, oblida el secret i deixa de funcionar.
	code, out := callJSON(t, s, (*Server).handleConnexioDisconnect, "POST", "/api/connexions/disconnect", `{"id":"`+id+`"}`)
	if code != 200 || out["revoked"] != true {
		t.Fatalf("disconnect: %d %v", code, out)
	}
	f.mu.Lock()
	nrev := len(f.revoked)
	f.mu.Unlock()
	if nrev == 0 {
		t.Fatal("la desconnexió ha de revocar els tokens al proveïdor")
	}
	if _, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id+`","tool":"list_notes"}`); err == nil || !strings.Contains(err.Error(), "disconnected") {
		t.Fatalf("desconnectat no es fa servir: %v", err)
	}
	if c, _ := s.checkConnexio(ctx, id); c.Status != "disconnected" {
		t.Fatalf("una comprovació no torna a obrir el que la persona ha desconnectat: %v", c.Status)
	}
	if _, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id2+`","tool":"list_notes"}`); err != nil {
		t.Fatalf("l'altre compte continua: %v", err)
	}

	// El proveïdor revoca la sessió: «cal tornar a iniciar sessió», no «error».
	f.mu.Lock()
	f.access, f.refresh = map[string]string{}, map[string]string{}
	f.mu.Unlock()
	if _, err := s.runConnectorTool(ctx, `{"action":"read","connection":"`+id2+`","tool":"list_notes"}`); err == nil || !strings.Contains(err.Error(), "request") {
		t.Fatalf("sessió caducada: %v", err)
	}
	if c, _ := s.connexioStore().Get(id2); c.Status != "needs_session" {
		t.Fatalf("estat: %v", c.Status)
	}

	// Cap secret a disc en clar, ni al registre ni a cap log d'events.
	for _, p := range []string{filepath.Join(s.windStateDir(), "connexions.json"), connexions.VaultDir()} {
		filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(path)
				assertNoSecrets(t, path, string(b))
			}
			return nil
		})
	}
	evs, _ := s.Hub().eventStore.AfterUser(0, 1000, s.usuari())
	for _, e := range evs {
		assertNoSecrets(t, "event", e.Text+string(e.Payload))
	}
}

func TestConnexioWithoutClientRegistrationSaysSoAndOffersBrowser(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	// Un servidor que no anuncia registre dinàmic ni té client_id configurat.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": "x", "authorization_endpoint": "https://x/a", "token_endpoint": "https://x/t"})
		default:
			http.Error(w, "unauthorized", 401)
		}
	}))
	t.Cleanup(srv.Close)
	s := hubTestServer(t)
	s.mcp = mcp.NewManager(map[string]mcp.Server{"crm": {URL: srv.URL + "/mcp"}}, mcp.Options{Store: mcp.NewTokenStore(filepath.Join(t.TempDir(), "m.json")), NoRegister: true})
	code, out := callJSON(t, s, (*Server).handleConnexioConnect, "POST", "/api/connexions/connect", `{"service":"mcp","provider":"crm"}`)
	if code != http.StatusBadGateway || out["code"] != "needs_client_id" {
		t.Fatalf("sense registre: %d %v", code, out)
	}
	if _, ok := out["browserPath"]; !ok {
		t.Fatalf("ha de dir si hi ha el camí del navegador: %v", out)
	}
}

func TestLegacyIMAPPasswordMigratesWithoutLoss(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	useFakeGmail(t, "app-pass-123")
	useFakeICS(t)
	s := hubTestServer(t)
	legacy := `{"calendars":[{"name":"Personal","url":"https://calendar.example.org/ical/private-cal-secret/basic.ics"}],` +
		`"mail":{"host":"imap.gmail.com","port":993,"user":"anna@example.org","password":"app-pass-123","folder":"INBOX"},` +
		`"briefing":{"enabled":false,"time":"07:30","wind":""},"timezone":"Europe/Madrid"}`
	if err := os.MkdirAll(s.windStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.connectionsPath(), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	list := listConnexions(t, s)
	if len(list) != 2 {
		t.Fatalf("calendari i bústia migrats: %v", list)
	}
	raw, _ := os.ReadFile(s.connectionsPath())
	assertNoSecrets(t, "connections.json migrat", string(raw))
	if !strings.Contains(string(raw), "Europe/Madrid") || !strings.Contains(string(raw), "07:30") {
		t.Fatalf("la resta del fitxer es conserva:\n%s", raw)
	}
	// Les eines continuen funcionant amb el secret del magatzem.
	out, err := s.runMailTool(`{"action":"unread"}`)
	if err != nil || !strings.Contains(out, "2 messages unread") {
		t.Fatalf("correu després de migrar: %v %s", err, out)
	}
	if _, err := s.runCalendarTool(context.Background(), `{"action":"today"}`); err != nil {
		t.Fatalf("calendari després de migrar: %v", err)
	}
	// /api/connections continua igual per als clients antics, sense secrets.
	get := httptest.NewRecorder()
	s.handleConnections(get, httptest.NewRequest("GET", "/api/connections", nil))
	if body := get.Body.String(); !strings.Contains(body, `"has_password":true`) || !strings.Contains(body, "calendar.example.org/…") {
		t.Fatalf("GET antic: %s", body)
	} else {
		assertNoSecrets(t, "GET /api/connections", body)
	}
	// Una segona bústia és una connexió nova, distingible.
	g2 := useFakeGmail(t, "app-pass-123")
	_ = g2
	if w := connPost(t, s, `{"mail":{"host":"imap.example.org","user":"feina@example.org","password":"app-pass-123"}}`); w.Code != 200 {
		t.Fatalf("segona bústia: %d %s", w.Code, w.Body)
	}
	if n := len(s.mailSecrets()); n != 2 {
		t.Fatalf("dues bústies: %d", n)
	}
	if out, err := s.runMailTool(`{"action":"unread","account":"feina@example.org"}`); err != nil || !strings.Contains(out, "unread") {
		t.Fatalf("triar bústia: %v %s", err, out)
	}
	// Una contrasenya que ja no val → «cal tornar a iniciar sessió».
	useFakeGmail(t, "una-altra")
	if _, err := s.runMailTool(`{"action":"unread","account":"anna@example.org"}`); err == nil {
		t.Fatal("login rebutjat")
	}
	for _, c := range listConnexions(t, s) {
		if c["accountLabel"] == "anna@example.org" && c["status"] != "needs_session" {
			t.Fatalf("estat després d'un login rebutjat: %v", c)
		}
	}
}

func TestLegacyMigrationKeepsTheFileWhenTheVaultFails(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", data)
	// «secrets» és un fitxer: el magatzem no es pot crear.
	if err := os.WriteFile(filepath.Join(data, "secrets"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	useFakeGmail(t, "app-pass-123")
	s := hubTestServer(t)
	legacy := `{"mail":{"host":"imap.gmail.com","port":993,"user":"anna@example.org","password":"app-pass-123"}}`
	_ = os.MkdirAll(s.windStateDir(), 0o700)
	_ = os.WriteFile(s.connectionsPath(), []byte(legacy), 0o600)
	if out, err := s.runMailTool(`{"action":"unread"}`); err != nil || !strings.Contains(out, "unread") {
		t.Fatalf("el correu continua funcionant: %v %s", err, out)
	}
	raw, _ := os.ReadFile(s.connectionsPath())
	if !strings.Contains(string(raw), "app-pass-123") {
		t.Fatal("sense magatzem no es pot esborrar la contrasenya antiga (es perdria)")
	}
}

func TestConnexionsSecondOwnerCannotSeeOrUse(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	f := newFakeProvider(t)
	useFakeGmail(t, "app-pass-123")
	h := testHubWithUsers(t)
	useNotesConnector(t, h.def, f)
	anna := h.SessionFor(&User{Name: "anna"}, DefaultSession)
	bernat := h.SessionFor(&User{Name: "bernat"}, DefaultSession)
	conn, _ := connectAccount(t, anna, f, `{"service":"mcp","provider":"notes"}`, "anna@example.org")
	id := conn["id"].(string)
	if w := connPost(t, anna, `{"mail":{"user":"anna@example.org","password":"app-pass-123"}}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}

	if got := listConnexions(t, bernat); len(got) != 0 {
		t.Fatalf("en Bernat no veu les connexions de l'Anna: %v", got)
	}
	for _, fn := range []func(*Server, http.ResponseWriter, *http.Request){(*Server).handleConnexioCheck, (*Server).handleConnexioDisconnect, (*Server).handleConnexioRemove, (*Server).handleConnexioUpdate, (*Server).handleConnexioConnect} {
		if code, _ := callJSON(t, bernat, fn, "POST", "/api/connexions/x", `{"id":"`+id+`","allowed":["send"]}`); code != 404 {
			t.Fatalf("en Bernat no pot tocar %s: %d", id, code)
		}
	}
	if _, err := bernat.runConnectorTool(context.Background(), `{"action":"read","connection":"`+id+`","tool":"list_notes"}`); err == nil {
		t.Fatal("en Bernat no pot fer servir el compte de l'Anna")
	}
	if _, err := bernat.runMailTool(`{"action":"unread"}`); err == nil {
		t.Fatal("en Bernat no llegeix el correu de l'Anna")
	}
	if c, _ := anna.connexioStore().Get(id); c.Status != "connected" {
		t.Fatalf("la connexió de l'Anna no s'ha tocat: %v", c.Status)
	}
	if len(bernat.connexioSources()) != 0 || len(anna.connexioSources()) != 2 {
		t.Fatal("fonts per a B6 per owner")
	}
}

// Auditoria del token store MCP global: els seus servidors porten les
// credencials de l'administrador i un altre compte no els pot fer servir.
func TestGlobalMCPToolsWithAdminTokensAreAdminOnly(t *testing.T) {
	dir := t.TempDir()
	store := mcp.NewTokenStore(filepath.Join(dir, "mcp-oauth.json"))
	m := mcp.NewManager(map[string]mcp.Server{"demo": {Command: "x"}, "crm": {URL: "https://crm.example.org/mcp"}, "keyed": {Command: "y", Env: map[string]string{"KEY": "k"}}}, mcp.Options{Store: store, NoRegister: true})
	h := testHubWithUsers(t)
	h.def.mcp = m
	admin := h.SessionFor(&User{Name: "admin", Admin: true}, DefaultSession)
	user := h.SessionFor(&User{Name: "anna"}, DefaultSession)
	if admin.mcpGate() != nil {
		t.Fatal("l'administrador ho fa servir tot")
	}
	if user.mcpGate() == nil {
		t.Fatal("un altre compte passa pel filtre")
	}
	if agent.ExtraAllowed(context.Background(), user.id, "mcp_crm_list") {
		t.Fatal("una eina desconeguda o d'un remot amb credencials no passa")
	}
	if !agent.ExtraAllowed(context.Background(), admin.id, "mcp_crm_list") {
		t.Fatal("l'administrador sí")
	}
	if !agent.ExtraAllowed(context.Background(), user.id, "read") {
		t.Fatal("les eines natives no es filtren")
	}
}

// Una feina espera una connexió (waiting_connection), el servidor es
// reinicia, la persona connecta el compte i la feina continua sola.
func connexioWaitModel(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		w.Header().Set("Content-Type", "text/event-stream")
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "tool" {
			b, _ := json.Marshal("Eina: " + fmt.Sprint(last.Content))
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
			return
		}
		args, _ := json.Marshal(map[string]string{"action": "request", "provider": "notes", "reason": "Per llegir les teves notes"})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"k1\",\"function\":{\"name\":\"connector\",\"arguments\":%s}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", wire)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func TestConnexioWaitSurvivesRestartAndResumesOnConnect(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	shortLive(t)
	f := newFakeProvider(t)
	model, bodies := connexioWaitModel(t)
	s := decisionServer(t, model)
	useNotesConnector(t, s, f)

	raw := windChat(t, s, "Mira les meves notes")
	if !strings.Contains(raw, `"type":"connection"`) || !strings.Contains(raw, `"waiting"`) {
		t.Fatalf("el torn s'havia d'aparcar esperant la connexió:\n%s", raw)
	}
	pend := pendingWindAsks(s)
	if len(pend) != 1 || pend[0].Type != "connection" || pend[0].Site != "mcp:notes" || pend[0].RunID == 0 {
		t.Fatalf("decisió de connexió: %+v", pend)
	}
	d := pend[0]
	if run, _ := s.Hub().Queue().Get(d.RunID); run.State != runs.WaitingUser {
		t.Fatalf("la feina espera: %+v", run)
	}
	waiting := false
	for _, e := range runEvents(t, s, d.RunID) {
		waiting = waiting || e.Kind == "run_waiting_connection"
	}
	if !waiting {
		t.Fatal("event run_waiting_connection")
	}
	if !strings.Contains(threadText(s), s.tr("Necessito que connectis", "I need you to connect")) {
		t.Fatalf("el fil diu què cal:\n%s", threadText(s))
	}

	// L'app es tanca i el servidor es reinicia mentre la persona connecta.
	s = restartServer(t, s)
	useNotesConnector(t, s, f)
	if n := s.CloseOrphanRuns(); n != 0 {
		t.Fatalf("una espera de connexió no és òrfena: %d", n)
	}
	conn, _ := connectAccount(t, s, f, `{"service":"mcp","provider":"notes"}`, "anna@example.org")
	if run := waitRun(t, s, d.RunID); run.State != runs.Completed {
		t.Fatalf("la feina continua sola en connectar: %+v", run)
	}
	thread := threadText(s)
	if !strings.Contains(thread, "The user connected "+conn["id"].(string)) || strings.Count(thread, "Mira les meves notes") != 1 {
		t.Fatalf("continua al mateix fil:\n%s", thread)
	}
	if p := pendingWindAsks(s); len(p) != 0 {
		t.Fatalf("res pendent: %+v", p)
	}
	for _, b := range *bodies {
		assertNoSecrets(t, "petició al model", b)
	}
	assertNoSecrets(t, "fil", thread)
}

// B6: fonts i canvis deterministes (calendari).
func TestConnexioChangesCalendarFeed(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	useFakeICS(t)
	s := hubTestServer(t)
	if w := connPost(t, s, `{"add_calendar":{"name":"Personal","url":"https://calendar.example.org/private-cal-secret.ics"}}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	srcs := s.connexioSources()
	if len(srcs) != 1 || !srcs[0].Changes || srcs[0].Status != "connected" {
		t.Fatalf("fonts: %+v", srcs)
	}
	_, cur, err := s.connexioChanges(context.Background(), srcs[0].ID, "")
	if err != nil || cur == "" {
		t.Fatalf("primera mirada: %v", err)
	}
	again, cur2, err := s.connexioChanges(context.Background(), srcs[0].ID, cur)
	if err != nil || len(again) != 0 || cur2 == "" {
		t.Fatalf("sense canvis no hi ha res nou: %v %+v", err, again)
	}
	assertNoSecrets(t, "cursor", cur2)
}
