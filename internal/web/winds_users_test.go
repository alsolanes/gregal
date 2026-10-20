package web

import (
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
)

// testHubWithUsers aixeca un hub de veritat (com el --serve en xarxa) amb
// usuàris opcionals: un Server default i sessions per usuari.
func testHubWithUsers(t *testing.T, names ...string) *Hub {
	t.Helper()
	cfg := &config.Config{
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"think":    {Provider: "p", Model: "m", ContextWindow: 32768},
			"code":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"reviewer": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	def := New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	return def.Hub()
}

// windsPathForTest exposa el camí dels vents per comprovar la separació.
func (s *Server) windsPathForTest() string { return s.winds.PathForTest() }

// TestPerUserWindState comprova que dues sessions d'usuaris diferents tenen
// vents i rotllanes PROPIS: el que crea l'un no ho veu l'altre, i la
// persistència queda a users/<nom>/.
func TestPerUserWindState(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	// Un hub de veritat amb dos usuaris, com fa el servidor en xarxa.
	hub := testHubWithUsers(t, "anna", "bert")

	anna := hub.SessionFor(&User{Name: "anna"}, "s1")
	bert := hub.SessionFor(&User{Name: "bert"}, "s1")

	// L'anna crea un vent; en Bert no l'ha de veure.
	if _, err := anna.winds.Save(companion.Wind{Name: "Tramuntana", Color: "#29ADFF", Role: "sistema"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := bert.winds.ByID("tramuntana"); ok {
		t.Error("bert veu el vent de l'anna: l'estat no és per usuari")
	}
	if _, ok := anna.winds.ByID("tramuntana"); !ok {
		t.Error("l'anna no veu el seu propi vent")
	}

	// Les rotllanes també són per usuari.
	group, err := anna.saveRotllana(Rotllana{Name: "Vesprada", Members: []string{"tramuntana"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bert.rotllanaByID(group.ID); ok {
		t.Error("bert veu la rotllana de l'anna")
	}
	if _, ok := anna.rotllanaByID(group.ID); !ok {
		t.Error("l'anna no veu la seva rotllana")
	}

	// La persistència cau a users/.identity-v1/<hash>/, no al directori compartit.
	if !strings.Contains(anna.windsPathForTest(), filepath.Join("users", ".identity-v1")) {
		t.Errorf("els vents de l'anna no són al seu directori: %s", anna.windsPathForTest())
	}
}

// TestPerUserSharedWhenNoUser: sense usuari, tot queda compartit com sempre
// (mode local d'un sol usuari): mateix directori per a dues sessions.
func TestPerUserSharedWhenNoUser(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)

	a := hub.SessionFor(nil, "s1")
	b := hub.SessionFor(nil, "s2")
	if _, err := a.winds.Save(companion.Wind{Name: "Xaloc", Color: "#FF9F00"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.winds.ByID("xaloc"); !ok {
		t.Error("sense usuaris l'estat ha de seguir sent compartit")
	}
}

// TestTothomKeepsSharedState: el token únic (mode local, Tothom) NO té estat
// per usuari: és l'arreplegat de sempre. Regressió del bug del mòbil, on les
// sessions amb token passaven a users/local/ i perdien els vents.
func TestTothomKeepsSharedState(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)

	// Tothom amb token: com el mòbil amb --token.
	tok := hub.SessionFor(&User{Name: "local", Admin: true, Tothom: true}, "mobil")
	if _, err := tok.winds.Save(companion.Wind{Name: "Tramuntana", Color: "#29ADFF"}); err != nil {
		t.Fatal(err)
	}
	// El mateix Tothom des d'una altra sessió ho veu: estat compartit.
	tok2 := hub.SessionFor(&User{Name: "local", Admin: true, Tothom: true}, "mobil2")
	if _, ok := tok2.winds.ByID("tramuntana"); !ok {
		t.Error("tothom ha perdut l'estat compartit: la sessió 2 no veu els vents")
	}
	// Els dirs de skills tampoc no són per usuari per al Tothom.
	if dirs := tok.userSkillsDirs(); dirs != nil {
		t.Errorf("tothom no ha de tenir skills pròpies: %v", dirs)
	}
}
