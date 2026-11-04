package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El perfil és de la persona: cada compte té el seu, sense límit de comptes, i
// ningú (ni un administrador) en pot llegir o escriure el d'un altre.

type profileBody struct {
	Prefs map[string]struct {
		V *string `json:"v"`
		T int64   `json:"t"`
	} `json:"prefs"`
	Keys []string `json:"keys"`
}

func profileCall(t *testing.T, s *Server, token, method, body string) (int, profileBody) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/profile", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var out profileBody
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestProfileIsPerPersonForManyAccounts(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", t.TempDir()+"/accounts")
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")

	// Cinc persones: el perfil de cadascuna és seu.
	tokens := map[string]string{}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("persona%d", i)
		if w := accountsPost(t, s, `{"name":"`+name+`","password":"12345678"}`); w.Code != 200 {
			t.Fatalf("create %s: %d", name, w.Code)
		}
		tok, _, err := s.users.Login(name, "12345678")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}
	for name, tok := range tokens {
		body := fmt.Sprintf(`{"prefs":{"gregal_tema":{"v":"tema-%s","t":1000},"gregal_idioma":{"v":"ca","t":1000}}}`, name)
		if code, _ := profileCall(t, s, tok, http.MethodPost, body); code != 200 {
			t.Fatalf("%s desa: %d", name, code)
		}
	}
	for name, tok := range tokens {
		code, p := profileCall(t, s, tok, http.MethodGet, "")
		if code != 200 {
			t.Fatalf("%s llegeix: %d", name, code)
		}
		got := p.Prefs["gregal_tema"].V
		if got == nil || *got != "tema-"+name {
			t.Fatalf("%s ha vist el perfil d'un altre: %v", name, got)
		}
		if len(p.Keys) == 0 {
			t.Fatal("la pàgina necessita la llista de claus")
		}
	}
	// L'administrador (clau de servidor) no veu el perfil de cap compte: té el seu.
	if _, p := profileCall(t, s, "admin-secret", http.MethodGet, ""); len(p.Prefs) != 0 {
		t.Fatalf("l'administrador no ha de veure perfils d'altres: %v", p.Prefs)
	}
}

func TestProfileMergeNewestWinsAndOnlyAllowedKeys(t *testing.T) {
	now := int64(10_000_000)
	str := func(v string) *string { return &v }
	cur := profileDoc{Prefs: map[string]profileEntry{"gregal_tema": {V: str("dark"), T: 5000}}}

	// Un canvi més antic no trepitja el més nou.
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_tema": {V: str("light"), T: 4000}}, now)
	if *cur.Prefs["gregal_tema"].V != "dark" {
		t.Fatal("un canvi vell no pot guanyar")
	}
	// Un de més nou, sí; dos dispositius amb claus diferents no es trepitgen.
	cur = mergeProfile(cur, map[string]profileEntry{
		"gregal_tema":     {V: str("calma"), T: 6000},
		"gregal_densitat": {V: str("compacta"), T: 100},
	}, now)
	if *cur.Prefs["gregal_tema"].V != "calma" || *cur.Prefs["gregal_densitat"].V != "compacta" {
		t.Fatalf("fusió: %v", cur.Prefs)
	}
	// Esborrar (v = null) també és un canvi.
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_tema": {V: nil, T: 7000}}, now)
	if e := cur.Prefs["gregal_tema"]; e.V != nil || e.T != 7000 {
		t.Fatalf("esborrat: %+v", e)
	}
	// El que no és de la llista (el token, una sessió) no hi entra mai, ni un valor enorme.
	cur = mergeProfile(cur, map[string]profileEntry{
		"gregal_token":   {V: str("secret"), T: 9000},
		"gregal_session": {V: str("abc"), T: 9000},
		"gregal_mida":    {V: str(strings.Repeat("x", 600)), T: 9000},
	}, now)
	for _, k := range []string{"gregal_token", "gregal_session", "gregal_mida"} {
		if _, ok := cur.Prefs[k]; ok {
			t.Fatalf("%s no havia d'entrar", k)
		}
	}
	// Una hora al futur llunyà no guanya per sempre: es talla a «ara».
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_view": {V: str("normal"), T: 1 << 60}}, now)
	if cur.Prefs["gregal_view"].T != now {
		t.Fatalf("hora futura: %d", cur.Prefs["gregal_view"].T)
	}
}

func TestProfileNeverCarriesSecretsOrDeviceState(t *testing.T) {
	for _, k := range []string{"gregal_token", "gregal_session", "gregal_draft_sessions", "gregal_resumed_conversations",
		"gregal_agent_browser_width", "gregal_preview_width", "gregal_side"} {
		if profileKeys[k] {
			t.Fatalf("%s no ha de viatjar amb el perfil", k)
		}
	}
}
