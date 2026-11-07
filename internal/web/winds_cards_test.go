package web

import (
	"context"
	"encoding/json"
	"errors"
	"gregal/internal/companion"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWindCardsPersistAndKeepLastSuccessfulUpdate(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	a := h.SessionFor(&User{Name: "anna"}, "one")
	if _, err := a.winds.Save(companion.Wind{Name: "Esports"}); err != nil {
		t.Fatal(err)
	}
	call := func(s *Server, method, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/api/winds/cards?wind=esports", strings.NewReader(body))
		s.handleWindCards(w, r)
		return w
	}
	created := call(a, "POST", `{"wind":"esports","action":"create","title":"Classificació","prompt":"Consulta la classificació"}`)
	if created.Code != 200 {
		t.Fatal(created.Body.String())
	}
	var card windCard
	if err := json.Unmarshal(created.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	old := windLoopStream
	defer func() { windLoopStream = old }()
	windLoopStream = func(_ *Server, _ context.Context, _ string, _ func(string, string), _ ...*routedRole) (exploracio, error) {
		return exploracio{text: "UPDATE_OK\nPrimers [font](https://example.com)"}, nil
	}
	body := `{"wind":"esports","action":"refresh","id":"` + card.ID + `"}`
	result := call(a, "POST", body)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	windLoopStream = func(_ *Server, _ context.Context, _ string, _ func(string, string), _ ...*routedRole) (exploracio, error) {
		return exploracio{}, errors.New("offline")
	}
	if result = call(a, "POST", body); result.Code != 502 {
		t.Fatalf("expected failure: %d", result.Code)
	}
	windLoopStream = func(_ *Server, _ context.Context, _ string, _ func(string, string), _ ...*routedRole) (exploracio, error) {
		return exploracio{text: "partial", parcial: true}, nil
	}
	if result = call(a, "POST", body); result.Code != 502 {
		t.Fatalf("partial update accepted: %d", result.Code)
	}
	restarted := New(h.def.cfg, h.def.cfgPath).Hub().SessionFor(&User{Name: "anna"}, "two")
	result = call(restarted, "GET", "")
	if result.Code != 200 || !strings.Contains(result.Body.String(), "Primers") {
		t.Fatalf("card lost: %s", result.Body.String())
	}
	if !strings.Contains(restarted.windCardsBrief("esports"), "Primers") {
		t.Fatal("wind cannot consult saved card")
	}
	b := New(h.def.cfg, h.def.cfgPath).Hub().SessionFor(&User{Name: "bert"}, "one")
	if _, err := b.winds.Save(companion.Wind{Name: "Esports"}); err != nil {
		t.Fatal(err)
	}
	result = call(b, "GET", "")
	if strings.Contains(result.Body.String(), "Primers") {
		t.Fatal("other account saw card")
	}
}
