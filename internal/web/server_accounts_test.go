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

}
