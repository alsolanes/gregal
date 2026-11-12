package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func testCredentials(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	raw, _ := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "proj-prova", "client_email": "sa@proj-prova.iam",
		"private_key": string(pemKey), "token_uri": tokenURI,
	})
	return raw
}

// TestFCMSendsDataOnly: el missatge és només de dades (cap bloc
// «notification» amb text), amb prioritat alta, i el token OAuth es
// reutilitza entre enviaments.
func TestFCMSendsDataOnly(t *testing.T) {
	var oauthCalls atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			oauthCalls.Add(1)
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || strings.Count(r.Form.Get("assertion"), ".") != 2 {
				http.Error(w, "bad", 400)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"acc-1","expires_in":3600}`)
		case "/send":
			if r.Header.Get("Authorization") != "Bearer acc-1" {
				http.Error(w, "auth", 401)
				return
			}
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(b))
			if strings.Contains(string(b), "tok-mort") {
				w.WriteHeader(404)
				_, _ = io.WriteString(w, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)
				return
			}
			_, _ = io.WriteString(w, `{"name":"projects/proj-prova/messages/1"}`)
		}
	}))
	defer srv.Close()
	f, err := NewFCM(testCredentials(t, srv.URL+"/token"))
	if err != nil {
		t.Fatal(err)
	}
	f.endpoint = srv.URL + "/send"
	ctx := context.Background()
	if err := f.Send(ctx, "tok-1", map[string]string{"k": "novetat", "id": "n-1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Send(ctx, "tok-mort", map[string]string{"k": "novetat", "id": "n-2"}); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("token mort = %v", err)
	}
	if oauthCalls.Load() != 1 {
		t.Fatalf("OAuth cridat %d vegades", oauthCalls.Load())
	}
	var msg struct {
		Message map[string]json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal([]byte(bodies[0]), &msg); err != nil {
		t.Fatal(err)
	}
	if _, ok := msg.Message["notification"]; ok {
		t.Fatalf("no ha de portar bloc notification: %s", bodies[0])
	}
	if !strings.Contains(bodies[0], `"priority":"high"`) || !strings.Contains(bodies[0], `"id":"n-1"`) {
		t.Fatalf("cos = %s", bodies[0])
	}
}

func TestNewFCMRejectsIncomplete(t *testing.T) {
	if _, err := NewFCM([]byte(`{"project_id":"p"}`)); err == nil {
		t.Fatal("credencials incompletes acceptades")
	}
	if _, err := NewFCM([]byte(`no json`)); err == nil || strings.Contains(err.Error(), "no json") {
		t.Fatalf("error = %v", err)
	}
}

func TestCredentialsPathFallsBackToDataDirFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", dir)
	t.Setenv(EnvCredentials, "")
	if got := CredentialsPath(); got != "" {
		t.Fatalf("sense fitxer ni variable: %q", got)
	}
	f := filepath.Join(dir, DefaultFile)
	if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := CredentialsPath(); got != f {
		t.Fatalf("fitxer per defecte: %q", got)
	}
	t.Setenv(EnvCredentials, "/altre.json")
	if got := CredentialsPath(); got != "/altre.json" {
		t.Fatalf("la variable mana: %q", got)
	}
}
