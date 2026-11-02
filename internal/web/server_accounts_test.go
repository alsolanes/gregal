package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedAccountPasswordMinimumIsEight(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", data)
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	handler := adminOnly((*Server).handleServerAccounts)
	create := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler(s, w, req)
		return w
	}
	// Set contrasenyes no han de passar ni crear el compte.
	if w := create(`{"name":"dina","password":"1234567"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("7 caràcters: codi %d, s'esperava 400", w.Code)
	}
	// Vuit sí: el mínim és vuit, com abans.
	if w := create(`{"name":"dina","password":"12345678"}`); w.Code != http.StatusOK {
		t.Fatalf("8 caràcters: codi %d (%s), s'esperava 200", w.Code, strings.TrimSpace(w.Body.String()))
	}
	if _, _, err := s.users.Login("dina", "12345678"); err != nil {
		t.Fatalf("el compte de 8 caràcters no pot entrar: %v", err)
	}
}

func TestManagedAccountsPersistAndRequireAdmin(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", data)
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	handler := adminOnly((*Server).handleServerAccounts)
	body := `{"name":"anna","password":"a-long-unique-password"}`
	req := httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUser{}, &User{Name: "bert"}))
	w := httptest.NewRecorder()
	handler(s, w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(body))
	w = httptest.NewRecorder()
	handler(s, w, req)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	// The real HTTP route must reject a non-administrator's bearer token.
	userToken, _, loginErr := s.users.Login("anna", "a-long-unique-password")
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	httpReq := httptest.NewRequest(http.MethodGet, "/api/server/accounts", nil)
	httpReq.Header.Set("Authorization", "Bearer "+userToken)
	httpReply := httptest.NewRecorder()
	s.Handler().ServeHTTP(httpReply, httpReq)
	if httpReply.Code != http.StatusForbidden {
		t.Fatalf("HTTP non-admin: %d", httpReply.Code)
	}
	raw, err := os.ReadFile(managedAccountsPath())
	if err != nil || strings.Contains(string(raw), "a-long-unique-password") {
		t.Fatal("password stored in plaintext or account missing")
	}
	token, _, err := s.users.Login("anna", "a-long-unique-password")
	if err != nil {
		t.Fatal(err)
	}
	again := New(h.def.cfg, h.def.cfgPath)
	if _, user, err := again.users.Login("anna", "a-long-unique-password"); err != nil || user.Admin || !strings.HasPrefix(user.Home, os.Getenv("GREGAL_ACCOUNTS_DIR")) {
		t.Fatalf("restart: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(body))
	w = httptest.NewRecorder()
	handler(s, w, req)
	if w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(`{"action":"reset","name":"anna","password":"another-long-password"}`))
	w = httptest.NewRecorder()
	handler(s, w, req)
	if w.Code != 200 {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	if _, ok := s.users.Resolve(token); ok {
		t.Fatal("old token still active")
	}
	after := New(h.def.cfg, h.def.cfgPath)
	if _, ok := after.users.Resolve(token); ok {
		t.Fatal("old token restored after restart")
	}
	if _, _, err := after.users.Login("anna", "a-long-unique-password"); err == nil {
		t.Fatal("old password accepted")
	}
	if _, _, err := after.users.Login("anna", "another-long-password"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	// Un cop el fitxer de tokens ja no té les sessions tancades, les
	// empremtes s'esborren: la llista no creix a cada canvi de contrasenya.
	if list, err := readManaged(); err != nil || len(list) != 1 || len(list[0].Revoked) != 0 {
		t.Fatalf("revoked digests kept: %+v %v", list, err)
	}
	if raw, _ := os.ReadFile(tokensPath()); strings.Contains(string(raw), token) {
		t.Fatal("closed session still in the tokens file")
	}
}

func accountsPost(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/server/accounts", strings.NewReader(body))
	w := httptest.NewRecorder()
	adminOnly((*Server).handleServerAccounts)(s, w, req)
	return w
}

func TestManagedAccountNamesIgnoreCase(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	if w := accountsPost(t, s, `{"name":"Anna","password":"12345678"}`); w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := accountsPost(t, s, `{"name":"anna","password":"12345678"}`); w.Code != http.StatusConflict {
		t.Fatalf("same name in other case: %d, want 409", w.Code)
	}
}

func TestManagedAccountDelete(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	accounts := filepath.Join(t.TempDir(), "accounts")
	t.Setenv("GREGAL_ACCOUNTS_DIR", accounts)
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	if w := accountsPost(t, s, `{"name":"pere","password":"12345678"}`); w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	home := managedDefinition(accounts, managedAccount{Name: "pere"}).Home
	if err := os.WriteFile(filepath.Join(home, "notes.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.users.Login("pere", "12345678")
	if err != nil {
		t.Fatal(err)
	}
	// Els comptes del config no es poden esborrar des d'aquí.
	if w := accountsPost(t, s, `{"name":"bert","action":"delete"}`); w.Code != 404 {
		t.Fatalf("config account delete: %d, want 404", w.Code)
	}
	if w := accountsPost(t, s, `{"name":"pere","action":"delete"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"archived":true`) {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if _, ok := s.users.Resolve(token); ok {
		t.Fatal("deleted account's session still active")
	}
	if _, _, err := s.users.Login("pere", "12345678"); err == nil {
		t.Fatal("deleted account can still sign in")
	}
	if list, _ := readManaged(); len(list) != 0 {
		t.Fatalf("account still in registry: %+v", list)
	}
	after := New(h.def.cfg, h.def.cfgPath)
	if _, ok := after.users.Resolve(token); ok {
		t.Fatal("deleted account's session restored after restart")
	}
	// Un compte nou amb el mateix nom comença de zero; les dades velles
	// queden apartades al disc.
	if w := accountsPost(t, s, `{"name":"pere","password":"87654321"}`); w.Code != 200 {
		t.Fatalf("recreate: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(home, "notes.txt")); !os.IsNotExist(err) {
		t.Fatal("new account inherited the old account's files")
	}
	old, _ := filepath.Glob(home + ".deleted-*")
	if len(old) != 1 {
		t.Fatalf("old data not archived: %v", old)
	}
}
