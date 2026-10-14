package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/mcp"
)

// fakeMCP és un servidor MCP remot mínim (Streamable HTTP); amb auth, demana
// OAuth i publica les metadades i el registre dinàmic.
func fakeMCP(t *testing.T, auth bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if auth && r.Header.Get("Authorization") != "Bearer ok" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+srv.URL+`/.well-known/oauth-protected-resource"`)
			http.Error(w, "no", 401)
			return
		}
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&m)
		if len(m.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		var res any
		switch m.Method {
		case "initialize":
			res = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "f"}}
		case "tools/list":
			res = map[string]any{"tools": []any{map[string]any{"name": "a", "inputSchema": map[string]any{"type": "object"}}, map[string]any{"name": "b"}}}
		default:
			res = map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": res})
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"resource": srv.URL + "/mcp", "authorization_servers": []string{srv.URL}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize", "token_endpoint": srv.URL + "/token", "registration_endpoint": srv.URL + "/register"})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"client_id": "cid"})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func mcpWebServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	s := hubTestServer(t)
	s.cfg.Agent.MaxSteps = 10
	s.cfg.Verify.Mode = "off"
	s.mcp = mcp.SetupWith(nil, mcp.Options{Store: mcp.NewTokenStore(filepath.Join(t.TempDir(), "tok.json")), NoRegister: true})
	t.Cleanup(s.mcp.Close)
	s.Token = "secret"
	return s, s.Handler()
}

func doReq(h http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "127.0.0.1:8097"
	if auth {
		r.Header.Set("Authorization", "Bearer secret")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMCPWebAfegirITreure(t *testing.T) {
	remote := fakeMCP(t, false)
	s, h := mcpWebServer(t)
	if w := doReq(h, "POST", "/api/mcp/servers", `{"name":"remot","url":"`+remote.URL+`/mcp"}`, false); w.Code != 401 {
		t.Fatalf("sense token: %d", w.Code)
	}
	w := doReq(h, "POST", "/api/mcp/servers", `{"name":"remot","url":"`+remote.URL+`/mcp","headers":{"X-Key":"${MY_KEY}"}}`, true)
	if w.Code != 200 {
		t.Fatalf("afegir: %d %s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(s.cfgPath)
	if err != nil || !strings.Contains(string(raw), remote.URL+"/mcp") || !strings.Contains(string(raw), "${MY_KEY}") {
		t.Fatalf("config desat amb la variable sense expandir: %s %v", raw, err)
	}
	w = doReq(h, "GET", "/api/mcp", "", true)
	var st struct {
		Servers []map[string]any `json:"servers"`
		Tools   []string         `json:"tools"`
	}
	json.Unmarshal(w.Body.Bytes(), &st)
	if len(st.Servers) != 1 || st.Servers[0]["status"] != "connected" || st.Servers[0]["tools"].(float64) != 2 || len(st.Tools) != 2 {
		t.Fatalf("estat: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "MY_KEY") {
		t.Fatalf("l'estat no exposa capçaleres: %s", w.Body.String())
	}
	if w := doReq(h, "POST", "/api/mcp/servers", `{"name":"x","command":""}`, true); w.Code != 400 {
		t.Fatalf("servidor invàlid: %d", w.Code)
	}
	w = doReq(h, "DELETE", "/api/mcp/servers?name=remot", "", true)
	if w.Code != 200 {
		t.Fatalf("treure: %d %s", w.Code, w.Body.String())
	}
	raw, _ = os.ReadFile(s.cfgPath)
	if strings.Contains(string(raw), "remot") {
		t.Fatalf("encara al config: %s", raw)
	}
}

func TestMCPWebConnectOAuthICallbackSenseToken(t *testing.T) {
	remote := fakeMCP(t, true)
	_, h := mcpWebServer(t)
	w := doReq(h, "POST", "/api/mcp/servers", `{"name":"segur","url":"`+remote.URL+`/mcp"}`, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "needs_auth") {
		t.Fatalf("afegir: %d %s", w.Code, w.Body.String())
	}
	w = doReq(h, "POST", "/api/mcp/connect", `{"name":"segur"}`, true)
	var res struct {
		AuthURL string `json:"auth_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	u, err := url.Parse(res.AuthURL)
	if w.Code != 200 || err != nil || u.Query().Get("redirect_uri") != "http://127.0.0.1:8097/api/mcp/oauth/callback" {
		t.Fatalf("connect: %d %s", w.Code, w.Body.String())
	}
	// El callback arriba del navegador sense token: no és un 401, és el
	// state el que decideix (aquí un de fals → 400).
	w = doReq(h, "GET", "/api/mcp/oauth/callback?state=fals&code=x", "", false)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "authorization failed") {
		t.Fatalf("callback: %d %s", w.Code, w.Body.String())
	}
}
