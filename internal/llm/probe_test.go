package llm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeModels(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":[{"id":"b-model"},{"id":"a-model"}]}`))
	}))
	defer srv.Close()
	ids, err := ProbeModels(srv.URL, "sk-x")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if len(ids) != 2 || ids[0] != "a-model" || ids[1] != "b-model" {
		t.Fatalf("ids=%v", ids)
	}
	if gotAuth != "Bearer sk-x" {
		t.Fatalf("auth=%q", gotAuth)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer bad.Close()
	if _, err := ProbeModels(bad.URL, "k"); err == nil {
		t.Fatal("404 hauria de fallar")
	}
}

func TestZenHint(t *testing.T) {
	h := zenHint("https://opencode.ai/zen/go/v1", 500, `{"message":"Internal server error"}`)
	if h == "" {
		t.Fatal("hauria d'afegir guia al 500 del Zen")
	}
	if zenHint("https://opencode.ai/zen/go/v1", 401, "Invalid API key.") != "" {
		t.Fatal("401 no porta hint")
	}
	if zenHint("http://127.0.0.1:8087/v1", 500, "Internal server error") != "" {
		t.Fatal("local no porta hint del Zen")
	}
}

// L'error de xarxa de Go és exacte i inútil per a qui només vol saber si
// el servidor està engegat. Es tradueix, i es conserva l'adreça.
func TestNetAmable(t *testing.T) {
	casos := []struct{ err, vol string }{
		{"dial tcp [::1]:8089: connectex: No connection could be made because the target machine actively refused it.", "no accepta connexions"},
		{"context deadline exceeded", "no respon a temps"},
		{"dial tcp: lookup nohi.example: no such host", "no existeix"},
	}
	for _, c := range casos {
		got := netAmable("http://localhost:8089/v1/models", errors.New(c.err)).Error()
		if !strings.Contains(got, c.vol) {
			t.Errorf("netAmable(%q) = %q, hi hauria de sortir %q", c.err, got, c.vol)
		}
		if !strings.Contains(got, "localhost:8089") {
			t.Errorf("l'adreça s'ha de conservar: %q", got)
		}
	}
}
