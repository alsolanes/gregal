package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rewriteTransport desvia les peticions al servidor de proves conservant el
// camí, per poder comprovar què s'envia a un amfitrió que no controlem.
type rewriteTransport struct {
	target string
	header http.Header
}

func (t *rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.header = r.Header.Clone()
	req, err := http.NewRequestWithContext(r.Context(), r.Method, t.target+r.URL.Path, r.Body)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	return http.DefaultTransport.RoundTrip(req)
}

// TestCapcaleraOpencodeSession: opencode.ai/zen/go encamina per sessió i sense
// aquesta capçalera respon "MissingSessionID".
func TestCapcaleraOpencodeSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()

	msgs := []Message{{Role: "user", Content: "hola"}}

	// Host qualsevol: cap capçalera pròpia.
	tr := &rewriteTransport{target: srv.URL}
	c := &Client{hc: &http.Client{Transport: tr}}
	if _, err := c.Chat(context.Background(), "https://exemple.test/v1", "", "m", msgs, 0, 10); err != nil {
		t.Fatal(err)
	}
	if got := tr.header.Get("x-opencode-session"); got != "" {
		t.Fatalf("no s'hauria d'enviar la capçalera a un host qualsevol: %q", got)
	}

	// opencode.ai: la capçalera hi és, i és estable entre crides.
	tr2 := &rewriteTransport{target: srv.URL}
	c2 := &Client{hc: &http.Client{Transport: tr2}}
	if _, err := c2.Chat(context.Background(), "https://opencode.ai/zen/go/v1", "k", "m", msgs, 0, 10); err != nil {
		t.Fatal(err)
	}
	primer := tr2.header.Get("x-opencode-session")
	if primer == "" {
		t.Fatal("falta la capçalera x-opencode-session")
	}
	if len(primer) != 24 {
		t.Fatalf("sessionID inesperat: %q", primer)
	}
	if _, err := c2.Chat(context.Background(), "https://opencode.ai/zen/go/v1", "k", "m", msgs, 0, 10); err != nil {
		t.Fatal(err)
	}
	if segon := tr2.header.Get("x-opencode-session"); segon != primer {
		t.Fatalf("la sessió hauria de ser estable: %q vs %q", primer, segon)
	}
	if got := tr2.header.Get("Authorization"); got != "Bearer k" {
		t.Fatalf("Authorization inesperat: %q", got)
	}
}
