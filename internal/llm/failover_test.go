package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func foServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

const foOK = `{"choices":[{"message":{"content":"hola","tool_calls":[]}}]}`

func TestIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{context.Canceled, false},
		{context.DeadlineExceeded, false},
		{fmt.Errorf("provider: %w", context.DeadlineExceeded), false},
		{fmt.Errorf(`provider http://x: HTTP 500: ups`), true},
		{fmt.Errorf(`provider http://x: HTTP 429: lent`), true},
		{fmt.Errorf(`provider http://x: HTTP 502: gateway`), true},
		{fmt.Errorf(`provider http://x: HTTP 500: Failed to parse tool call arguments as JSON: parse_error.101`), true},
		{fmt.Errorf(`provider http://x: HTTP 400: malament`), false},
		{fmt.Errorf(`provider http://x: HTTP 401: sense clau`), false},
		{fmt.Errorf(`provider http://x: HTTP 403: prohibit`), false},
		{fmt.Errorf(`provider http://x: HTTP 404: no hi és`), false},
		{fmt.Errorf(`provider http://127.0.0.1:9: connection refused`), true},
		{nil, false},
	} {
		if got := IsRetryable(tc.err); got != tc.want {
			t.Errorf("IsRetryable(%v)=%v, volia %v", tc.err, got, tc.want)
		}
	}
}

func TestFailoverPrimariCau(t *testing.T) {
	bad := foServer(t, 500, "error intern")
	defer bad.Close()
	good := foServer(t, 200, foOK)
	defer good.Close()
	c := New()
	prim := Target{BaseURL: bad.URL, Model: "caigut"}
	fb := &Target{BaseURL: good.URL, Model: "reserva"}
	out, _, used, err := c.ChatWithToolsFO(context.Background(), prim, fb, []Message{{Role: "user", Content: "hola"}}, 0, 100, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !used || out != "hola" {
		t.Fatalf("used=%v out=%q", used, out)
	}
}

func TestFailoverNoAmaga400(t *testing.T) {
	bad := foServer(t, 400, "petició malformada")
	defer bad.Close()
	good := foServer(t, 200, foOK)
	defer good.Close()
	c := New()
	prim := Target{BaseURL: bad.URL, Model: "mal-configurat"}
	fb := &Target{BaseURL: good.URL, Model: "reserva"}
	_, _, used, err := c.ChatWithToolsFO(context.Background(), prim, fb, []Message{{Role: "user", Content: "hola"}}, 0, 100, nil, nil)
	if err == nil || used {
		t.Fatalf("el 400 no ha de provar fallback: used=%v err=%v", used, err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("err=%v", err)
	}
}

func TestFailoverTotsDosCauen(t *testing.T) {
	b1 := foServer(t, 500, "a")
	defer b1.Close()
	b2 := foServer(t, 500, "b")
	defer b2.Close()
	c := New()
	_, _, _, err := c.ChatWithToolsFO(context.Background(),
		Target{BaseURL: b1.URL, Model: "u"}, &Target{BaseURL: b2.URL, Model: "dos"},
		[]Message{{Role: "user", Content: "hola"}}, 0, 100, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "dos") {
		t.Fatalf("cal error dels dos: %v", err)
	}
}

func TestSenseFallbackIgualQueAbans(t *testing.T) {
	good := foServer(t, 200, foOK)
	defer good.Close()
	c := New()
	out, _, used, err := c.ChatWithToolsFO(context.Background(),
		Target{BaseURL: good.URL, Model: "m"}, nil,
		[]Message{{Role: "user", Content: "hola"}}, 0, 100, nil, nil)
	if err != nil || used || out != "hola" {
		t.Fatalf("out=%q used=%v err=%v", out, used, err)
	}
}
