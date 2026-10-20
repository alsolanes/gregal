package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQ1HubTupleCollisionAndListing(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	a, b := &User{Name: "anna"}, &User{Name: "anna-b"}
	first := h.SessionFor(a, "b-mobil")
	second := h.SessionFor(b, "mobil")
	local := h.Session("anna-b-mobil")
	if first == second || first == local || second == local {
		t.Error("distinct owner/session tuples selected the same server")
	}
	if first.user != a || second.user != b {
		t.Error("session resolved with another principal")
	}
	for _, u := range []*User{a, b} {
		if list := h.listFor(u); len(list) != 1 {
			t.Errorf("%q listed foreign sessions: %+v", u.Name, list)
		}
	}
	if !h.CloseFor(b, "mobil") {
		t.Error("could not close own session")
	}
	if h.SessionFor(a, "b-mobil") != first {
		t.Error("closing another principal's session removed ours")
	}
}

func TestQ1HubDoesNotStripRawUsernamePrefix(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	u := &User{Name: "anna"}
	a, b := h.SessionFor(u, "anna-mobil"), h.SessionFor(u, "mobil")
	if a == b {
		t.Fatal("raw session ID lost a username-looking prefix")
	}
	for _, s := range []*Server{a, b} {
		// Round-trip via the WIRE id (what clients send back), not the
		// internal key: the two must never alias each other's session.
		if h.SessionFor(u, s.wireID()) != s {
			t.Errorf("wire ID did not round trip: %q", s.wireID())
		}
	}
	if h.SessionFor(u, "anna-mobil") == h.SessionFor(u, "mobil") {
		t.Error("distinct wire IDs aliased the same session")
	}
}

func TestQ1HubTothomAndNamedPrincipalAreDistinct(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	legacy, named := &User{Name: "local", Tothom: true}, &User{Name: "local"}
	a, b := h.SessionFor(legacy, "mobile"), h.SessionFor(named, "mobile")
	if a == b {
		t.Fatal("Tothom aliases a named account")
	}
	if a.goalDir != h.def.goalDir || b.goalDir == a.goalDir {
		t.Fatal("shared and personal stores not separated")
	}
	if len(h.listFor(legacy)) != 1 || len(h.listFor(named)) != 1 {
		t.Fatal("listing aliases Tothom and named identities")
	}
}

func TestQ1HubWireIDRoundTripsThroughHTTP(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	u := &User{Name: "Ànna_b-" + strings.Repeat("x", 120)}
	handler := h.def.HandlerWithIdentity(func(*http.Request) (*User, error) { return u, nil })
	call := func(method, path, body, session string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if session != "" {
			r.Header.Set("X-Gregal-Session", session)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	raw := strings.Repeat("s", 64)
	opened := call(http.MethodPost, "/api/sessions/open", `{"id":"`+raw+`","title":"my private session"}`, "")
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(opened.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if h.SessionFor(u, got.ID) != h.SessionFor(u, raw) {
		t.Fatal("returned ID selects another server")
	}
	for _, query := range []bool{false, true} {
		// The wire ID must select the SAME server (same live session,
		// same title) both as header and as ?session= query.
		path, id := "/api/sessions/live", got.ID
		if query {
			path += "?session=" + got.ID
			id = ""
		}
		live := call(http.MethodGet, path, "", id)
		var sess struct {
			Sessions []map[string]any `json:"sessions"`
			Current  string           `json:"current"`
		}
		if err := json.Unmarshal(live.Body.Bytes(), &sess); err != nil {
			t.Fatal(err)
		}
		if sess.Current != got.ID {
			t.Fatalf("wire ID lost selected session: current=%q want %q", sess.Current, got.ID)
		}
		found := false
		for _, item := range sess.Sessions {
			if item["id"] == got.ID && item["title"] == "my private session" {
				found = true
			}
		}
		if !found {
			t.Fatalf("wire ID lost selected session: %+v", sess)
		}
	}
	call(http.MethodPost, "/api/sessions/open", `{"id":"`+got.ID+`"}`, "")
	call(http.MethodGet, "/api/v2/events?session="+got.ID, "", got.ID)
	if len(h.listFor(u)) != 1 {
		t.Fatal("wire ID created phantom sessions")
	}
	foreign := &User{Name: "another-account"}
	if h.SessionFor(foreign, got.ID) == h.SessionFor(u, raw) {
		t.Fatal("foreign wire ID selected original owner")
	}
	call(http.MethodPost, "/api/sessions/close", `{"id":"`+got.ID+`"}`, "")
	if len(h.listFor(u)) != 0 {
		t.Fatal("close by wire ID left session alive")
	}
}
