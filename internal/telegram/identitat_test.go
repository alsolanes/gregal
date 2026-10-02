package telegram

import (
	"context"
	"path/filepath"
	"testing"

	"gregal/internal/config"
)

// TestIdentitatPerUsuari: un id lligat resol compte, casa i política
// pròpia; un id sense lligar cau al projecte global.
func TestIdentitatPerUsuari(t *testing.T) {
	dir := t.TempDir()
	aleixHome := dir + "/usera"
	cfg := &config.Config{
		Language:  "ca",
		Providers: map[string]config.Provider{},
		Roles:     map[string]config.Role{},
		Mode:      "code",
		Users: map[string]config.UserCfg{
			"usera": {Roots: []string{"/"}, Home: aleixHome, Admin: true, TelegramID: 1234567},
			"userb": {Roots: []string{dir + "/userb"}, Home: dir + "/userb", TelegramID: 777},
		},
	}
	b := New(Options{Cfg: cfg, Cwd: dir, Token: "123:abc", AllowedUsers: []int64{1234567}})

	user, casa, pol := b.identity(1234567)
	if user != "usera" || casa != aleixHome {
		t.Fatalf("usera: user=%q casa=%q", user, casa)
	}
	if pol == nil || pol.ProjectDir != aleixHome {
		t.Fatalf("usera: política sense ProjectDir propi")
	}
	// La política es desa: dues crides tornen la mateixa.
	if _, _, pol2 := b.identity(1234567); pol2 != pol {
		t.Fatal("la política per usuari s'ha de reutilitzar")
	}

	user, casa, pol = b.identity(777)
	if user != "userb" || casa != dir+"/userb" {
		t.Fatalf("userb: user=%q casa=%q", user, casa)
	}

	// Sense lligam: projecte global, com abans.
	user, casa, pol = b.identity(999)
	if user != "" || casa != dir || pol != b.policy {
		t.Fatalf("convidat: user=%q casa=%q (volia global)", user, casa)
	}

	// Autoritzats: lligats entren sense ser a allowed_users.
	if !b.authorized(&User{ID: 777}, Chat{ID: 1, Type: "private"}) {
		t.Fatal("userb lligada ha d'entrar")
	}
	if b.authorized(&User{ID: 999}, Chat{ID: 1, Type: "private"}) {
		t.Fatal("desconegut no ha d'entrar")
	}
}

// El mode autònom fa anar les eines sense cap aprovació: tot i que el botó
// ja passa per canApprove, el text no ho feia. Un usuari familiar lligat
// (autoritzat, però fora d'allowed_users) no hi ha d'entrar de cap manera;
// l'amo sí.
func TestModeAutonomNomesPerAlQuiToca(t *testing.T) {
	tg := newFakeTG(t)
	dir := t.TempDir()
	cfg := &config.Config{
		Language:  "ca",
		Providers: map[string]config.Provider{},
		Roles:     map[string]config.Role{},
		Mode:      "code",
		Users: map[string]config.UserCfg{
			"usera": {Roots: []string{"/"}, Home: dir, Admin: true, TelegramID: 1234567},
			"userb": {Roots: []string{dir}, Home: dir, TelegramID: 777},
		},
	}
	bot := New(Options{Cfg: cfg, CfgPath: filepath.Join(dir, "config.yaml"), Cwd: dir,
		Token: "123:abc", AllowedUsers: []int64{1234567},
		SessionsDir: filepath.Join(dir, "sessions"), GoalsDir: filepath.Join(dir, "goals")})
	bot.api = &API{Token: "123:abc", BaseURL: tg.srv.URL, HTTP: tg.srv.Client()}

	if !bot.mayRunUnattended(1234567) {
		t.Fatal("l'amo ha de poder entrar al mode autònom")
	}
	if bot.mayRunUnattended(777) {
		t.Fatal("un usuari lligat fora d'allowed_users no hi ha de poder entrar")
	}

	ctx := context.Background()
	bot.handleMessage(ctx, msg(1234567, "/mode autonomous"))
	if got := bot.state(1234567).mode; got != "autonomous" {
		t.Fatalf("l'amo hauria d'haver entrat: mode=%q", got)
	}

	bot.handleMessage(ctx, msg(777, "/mode autonomous"))
	if got := bot.state(777).mode; got == "autonomous" {
		t.Fatalf("userb no hauria d'haver entrat per text: mode=%q", got)
	}
	if _, ok := tg.find("només el pot activar l'amo"); !ok {
		t.Fatalf("cal dir per què no hi pot entrar: %+v", tg.all())
	}
	// I pel botó (aquí ja hi ha canApprove, però el resultat ha de ser el mateix).
	bot.handleCallback(ctx, cb(777, 777, "mode:autonomous"))
	if got := bot.state(777).mode; got == "autonomous" {
		t.Fatalf("userb no hauria d'haver entrat pel botó: mode=%q", got)
	}
}
