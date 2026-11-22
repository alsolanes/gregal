package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type windsCatalog struct {
	AvatarTheme string `json:"avatarTheme"`
	Winds       []struct {
		ID     string `json:"id"`
		Avatar string `json:"avatar"`
	} `json:"winds"`
}

func getWinds(t *testing.T, s *Server) windsCatalog {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWinds(rec, httptest.NewRequest(http.MethodGet, "/api/winds", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /api/winds: %d %s", rec.Code, rec.Body.String())
	}
	var out windsCatalog
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func avatarOf(t *testing.T, s *Server, id string) string {
	t.Helper()
	for _, w := range getWinds(t, s).Winds {
		if w.ID == id {
			return w.Avatar
		}
	}
	t.Fatalf("no wind %s", id)
	return ""
}

func setAvatarTheme(t *testing.T, s *Server, theme string) int {
	t.Helper()
	return postJSON(t, s.handleWindsAvatarTheme, `{"theme":"`+theme+`"}`).Code
}

// Sense triar res, les cares de sempre; el tema es desa i un pack que no
// existeix no.
func TestAvatarThemeDefaultsToCaresAndRejectsUnknownPacks(t *testing.T) {
	s := rutinaTestServer(t)
	if got := getWinds(t, s).AvatarTheme; got != "cares" {
		t.Fatalf("tema per defecte: %q", got)
	}
	for _, theme := range []string{"esperit", "free", "cares"} {
		if code := setAvatarTheme(t, s, theme); code != 200 {
			t.Fatalf("tema %s: %d", theme, code)
		}
		if got := getWinds(t, s).AvatarTheme; got != theme {
			t.Fatalf("desat %s, llegit %q", theme, got)
		}
	}
	if code := setAvatarTheme(t, s, "mapa-del-tresor"); code != http.StatusBadRequest {
		t.Fatalf("un pack desconegut ha de ser 400: %d", code)
	}
	if got := getWinds(t, s).AvatarTheme; got != "cares" {
		t.Fatalf("un POST rebutjat no canvia el tema: %q", got)
	}
}

// Cada vent desa el seu pack, l'editor antic que no l'envia no l'esborra, i
// un pack desconegut és 400.
func TestWindAvatarIsSavedAndKeptWhenOmitted(t *testing.T) {
	s := rutinaTestServer(t)
	if got := avatarOf(t, s, "xaloc"); got != "" {
		t.Fatalf("sense triar, buit (cares): %q", got)
	}
	if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"esperit"}`); rec.Code != 200 {
		t.Fatalf("desa avatar: %d %s", rec.Code, rec.Body.String())
	}
	if got := avatarOf(t, s, "xaloc"); got != "esperit" {
		t.Fatalf("avatar desat: %q", got)
	}
	if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","followups":"off"}`); rec.Code != 200 {
		t.Fatalf("desa sense avatar: %d %s", rec.Code, rec.Body.String())
	}
	if got := avatarOf(t, s, "xaloc"); got != "esperit" {
		t.Fatalf("sense avatar al POST, es conserva: %q", got)
	}
	if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"bufador"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("pack desconegut: %d", rec.Code)
	}
}

// Canviar el tema no toca el pack de cada vent: tornar a «Lliure» el recupera.
func TestAvatarThemeKeepsEachWindsOwnAvatar(t *testing.T) {
	s := rutinaTestServer(t)
	if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"esperit"}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	for _, theme := range []string{"cares", "esperit", "free"} {
		if code := setAvatarTheme(t, s, theme); code != 200 {
			t.Fatalf("tema %s: %d", theme, code)
		}
		if got := avatarOf(t, s, "xaloc"); got != "esperit" {
			t.Fatalf("amb el tema %s, el vent ha perdut el seu pack: %q", theme, got)
		}
	}
}

// El tema és del compte: el que tria l'Anna no el veu en Bert.
func TestAvatarThemeIsPerAccount(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t, "anna", "bert")
	anna := hub.SessionFor(&User{Name: "anna"}, "s1")
	bert := hub.SessionFor(&User{Name: "bert"}, "s1")
	if code := setAvatarTheme(t, anna, "esperit"); code != 200 {
		t.Fatalf("anna: %d", code)
	}
	if got := getWinds(t, bert).AvatarTheme; got != "cares" {
		t.Fatalf("en Bert veu el tema de l'Anna: %q", got)
	}
	if got := getWinds(t, anna).AvatarTheme; got != "esperit" {
		t.Fatalf("l'Anna no veu el seu tema: %q", got)
	}
}

// Cap de vent i Rosa de rellotge són packs com els altres: valen per al tema
// i per a cada vent; un nom semblant que no ho és continua sent 400.
func TestCapAndRosaAreAvatarPacks(t *testing.T) {
	s := rutinaTestServer(t)
	for _, pack := range []string{"cap", "rosa"} {
		if code := setAvatarTheme(t, s, pack); code != 200 {
			t.Fatalf("tema %s: %d", pack, code)
		}
		if got := getWinds(t, s).AvatarTheme; got != pack {
			t.Fatalf("desat %s, llegit %q", pack, got)
		}
		if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"`+pack+`"}`); rec.Code != 200 {
			t.Fatalf("avatar %s: %d %s", pack, rec.Code, rec.Body.String())
		}
		if got := avatarOf(t, s, "xaloc"); got != pack {
			t.Fatalf("avatar desat %s, llegit %q", pack, got)
		}
	}
	for _, unknown := range []string{"bufador", "rellotge"} {
		if code := setAvatarTheme(t, s, unknown); code != http.StatusBadRequest {
			t.Fatalf("tema %s ha de ser 400: %d", unknown, code)
		}
		if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"`+unknown+`"}`); rec.Code != http.StatusBadRequest {
			t.Fatalf("avatar %s ha de ser 400: %d", unknown, rec.Code)
		}
	}
}

// El Mapa de vent és un pack com els altres: tema del compte i pack d'un vent.
func TestMapaIsAnAvatarPack(t *testing.T) {
	s := rutinaTestServer(t)
	if code := setAvatarTheme(t, s, "mapa"); code != 200 {
		t.Fatalf("tema mapa: %d", code)
	}
	if got := getWinds(t, s).AvatarTheme; got != "mapa" {
		t.Fatalf("tema desat: %q", got)
	}
	if rec := postJSON(t, s.handleWinds, `{"id":"xaloc","name":"Xaloc","avatar":"mapa"}`); rec.Code != 200 {
		t.Fatalf("avatar mapa: %d %s", rec.Code, rec.Body.String())
	}
	if got := avatarOf(t, s, "xaloc"); got != "mapa" {
		t.Fatalf("avatar desat: %q", got)
	}
}
