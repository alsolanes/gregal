package web

import (
	"encoding/json"
	"net/http"
	"os"

	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestInvitationSingleUseAndLogin(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	handler := s.Handler()
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call(http.MethodPost, "/api/server/invitations", "", ""); w.Code != 401 {
		t.Fatalf("unauthorized invite: %d", w.Code)
	}
	w := call(http.MethodPost, "/api/server/invitations", "admin-secret", "")
	if w.Code != 200 {
		t.Fatalf("invite: %d %s", w.Code, w.Body.String())
	}
	var invite struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &invite); err != nil || len(invite.Code) != 32 {
		t.Fatalf("invalid invite: %v", err)
	}
	// The invitation is stored as a hash, never in plaintext.
	raw, err := os.ReadFile(filepath.Join(invitationDir(), tokenDigest(invite.Code)))
	if err != nil || strings.Contains(string(raw), invite.Code) {
		t.Fatal("invitation not safely persisted")
	}
	badBody := `{"code":"` + invite.Code + `","name":"a","password":"a-unique-test-password"}`
	if w = call(http.MethodPost, "/api/register", "", badBody); w.Code != 400 {
		t.Fatalf("invalid name: %d", w.Code)
	}
	body := `{"code":"` + invite.Code + `","name":"anna","password":"a-unique-test-password"}`
	if w = call(http.MethodPost, "/api/register", "", body); w.Code != 200 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	if _, _, err := s.users.Login("anna", "a-unique-test-password"); err != nil {
		t.Fatalf("login: %v", err)
	}
	body = `{"code":"` + invite.Code + `","name":"bert","password":"a-unique-test-password"}`
	if w = call(http.MethodPost, "/api/register", "", body); w.Code != 401 {
		t.Fatalf("reused invite: %d %s", w.Code, w.Body.String())
	}
	expired := call(http.MethodPost, "/api/server/invitations", "admin-secret", "")
	if err := json.Unmarshal(expired.Body.Bytes(), &invite); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invitationDir(), tokenDigest(invite.Code)), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	body = `{"code":"` + invite.Code + `","name":"bert","password":"a-unique-test-password"}`
	if w = call(http.MethodPost, "/api/register", "", body); w.Code != 401 {
		t.Fatalf("expired invite: %d", w.Code)
	}

}

func TestInvitationConcurrentSingleUse(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", filepath.Join(t.TempDir(), "accounts"))
	s := testHubWithUsers(t).def
	s.SetToken("admin-secret")
	h := s.Handler()
	issue := httptest.NewRequest(http.MethodPost, "/api/server/invitations", nil)
	issue.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, issue)
	var inv struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inv); err != nil || inv.Code == "" {
		t.Fatal("no code")
	}
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, name := range []string{"anna", "bert"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			body := `{"code":"` + inv.Code + `","name":"` + name + `","password":"long-password-123"}`
			r := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
			reply := httptest.NewRecorder()
			h.ServeHTTP(reply, r)
			results <- reply.Code
		}(name)
	}
	wg.Wait()
	close(results)
	success := 0
	for status := range results {
		if status == 200 {
			success++
		} else if status != 401 {
			t.Fatalf("unexpected status: %d", status)
		}
	}
	if success != 1 {
		t.Fatalf("%d registrations", success)
	}
}
