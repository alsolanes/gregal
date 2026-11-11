package tools

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/identity"
)

// ambPerfilNet: perfil base a una carpeta temporal i registres nets.
func ambPerfilNet(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("GREGAL_BROWSER_PROFILE", base)
	t.Setenv("GREGAL_BROWSER_CDP", "")
	t.Setenv("GREGAL_ORDINADOR_OWNER", "")
	t.Setenv("GREGAL_ORDINADOR_VNC", "")
	t.Setenv("GREGAL_ORDINADOR_PROFILE", "")
	oldAttach := browserAttach
	browserAttach = ""
	reset := func() {
		migrateMu.Lock()
		migrateDone = map[string]bool{}
		migrateMu.Unlock()
		ownerChromeMu.Lock()
		ownerChromes = map[string]*ownerChrome{}
		ordinadorAttachUsers = map[string]int{}
		ownerChromeMu.Unlock()
	}
	reset()
	t.Cleanup(func() {
		CloseBrowser()
		browserAttach = oldAttach
		reset()
	})
	return base
}

func userScope(name, id string) string { return identity.Key(name) + "-" + id }

func TestBrowserOwnerKey(t *testing.T) {
	anna := identity.Key("anna")
	cases := map[string]string{
		"":                            OrdinadorLocal,
		"default":                     OrdinadorLocal,
		"agent":                       OrdinadorLocal,
		anna:                          OrdinadorLocal, // sense guió no és una clau d'usuari
		userScope("anna", "default"):  anna,
		userScope("anna", "mobil-2"):  anna,
		userScope("bernat", "mobil"):  identity.Key("bernat"),
		strings.ToUpper(anna) + "-x":  OrdinadorLocal,
		"x" + userScope("anna", "id"): OrdinadorLocal,
	}
	for scope, want := range cases {
		if got := BrowserOwnerKey(scope); got != want {
			t.Errorf("BrowserOwnerKey(%q) = %q, want %q", scope, got, want)
		}
	}
}

// El perfil és del propietari: dues feines d'Anna, el mateix; Bernat, un
// altre; el mode local, el perfil base de sempre.
func TestOrdinadorProfilePerOwner(t *testing.T) {
	base := ambPerfilNet(t)
	a1, err := OrdinadorProfileDir(BrowserOwnerKey(userScope("anna", "default")))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := OrdinadorProfileDir(BrowserOwnerKey(userScope("anna", "feina-viatge")))
	b1, _ := OrdinadorProfileDir(BrowserOwnerKey(userScope("bernat", "default")))
	local, _ := OrdinadorProfileDir(BrowserOwnerKey("default"))
	if a1 != a2 {
		t.Fatalf("dues feines de la mateixa persona han de compartir perfil: %s vs %s", a1, a2)
	}
	if a1 == b1 || strings.HasPrefix(b1, a1) || strings.HasPrefix(a1, b1) {
		t.Fatalf("dos usuaris no poden compartir perfil: %s vs %s", a1, b1)
	}
	if local != base {
		t.Fatalf("local ha de ser el perfil base: %s", local)
	}
	for _, d := range []string{a1, b1} {
		if filepath.Dir(d) != filepath.Join(base, "ordinadors") {
			t.Fatalf("perfil fora d'ordinadors/: %s", d)
		}
	}
}

func fakeProfile(t *testing.T, dir, marca string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "Local State"), []byte(marca), 0o600)
	os.WriteFile(filepath.Join(dir, "Default", "Cookies"), []byte(marca), 0o600)
}

func readMarca(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "Default", "Cookies"))
	if err != nil {
		return ""
	}
	return string(b)
}

// Migració: el login de la sessió default d'Anna passa al seu ordinador; el
// de Bernat no hi va a parar mai, i les altres sessions queden intactes.
func TestOrdinadorProfileMigration(t *testing.T) {
	base := ambPerfilNet(t)
	annaOld := legacySessionProfile(userScope("anna", "default"))
	annaAltre := legacySessionProfile(userScope("anna", "feina"))
	bernatOld := legacySessionProfile(userScope("bernat", "default"))
	localOld := legacySessionProfile("default")
	fakeProfile(t, annaOld, "anna")
	fakeProfile(t, annaAltre, "anna-feina")
	fakeProfile(t, bernatOld, "bernat")
	fakeProfile(t, localOld, "local")

	anna, err := OrdinadorProfileDir(identity.Key("anna"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readMarca(t, anna); got != "anna" {
		t.Fatalf("el perfil d'Anna havia de venir de la seua sessió default: %q", got)
	}
	if _, err := os.Stat(annaOld); !os.IsNotExist(err) {
		t.Fatalf("el perfil antic s'havia de moure, no copiar: %v", err)
	}
	if readMarca(t, annaAltre) != "anna-feina" || readMarca(t, bernatOld) != "bernat" {
		t.Fatal("la migració ha tocat perfils que no eren de la sessió default d'Anna")
	}
	// Segona vegada (o un altre procés): res a fer, res trepitjat.
	migrateDone = map[string]bool{}
	fakeProfile(t, annaOld, "anna-nou")
	if _, err := OrdinadorProfileDir(identity.Key("anna")); err != nil {
		t.Fatal(err)
	}
	if readMarca(t, anna) != "anna" {
		t.Fatal("un perfil ja migrat no es torna a sobreescriure")
	}
	// Local: al perfil base, sense trepitjar sessions/ ni ordinadors/.
	local, _ := OrdinadorProfileDir(OrdinadorLocal)
	if local != base || readMarca(t, local) != "local" {
		t.Fatalf("migració local: %s %q", local, readMarca(t, local))
	}
	if readMarca(t, bernatOld) != "bernat" {
		t.Fatal("la migració local ha tocat el perfil de Bernat")
	}
}

// El base local ja era un perfil (el del CLI): no es migra res al damunt.
func TestOrdinadorProfileLocalKeepsExisting(t *testing.T) {
	base := ambPerfilNet(t)
	fakeProfile(t, base, "cli")
	fakeProfile(t, legacySessionProfile("default"), "web")
	local, _ := OrdinadorProfileDir(OrdinadorLocal)
	if readMarca(t, local) != "cli" || readMarca(t, legacySessionProfile("default")) != "web" {
		t.Fatal("un perfil existent no es barreja amb l'antic")
	}
}

// llançadorPropietari: el Chrome de cada propietari és el CDP fals; apunta
// amb quin perfil s'engega i quantes vegades es tanca.
func llançadorPropietari(t *testing.T) (starts *[]string, kills *int) {
	t.Helper()
	f := newFakeCDP(t)
	t.Cleanup(f.srv.Close)
	_, portS, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	var port int
	for _, c := range portS {
		port = port*10 + int(c-'0')
	}
	t.Setenv("GREGAL_BROWSER", "fake-chrome")
	var mu sync.Mutex
	var s []string
	var k int
	old := startOwnerChrome
	startOwnerChrome = func(exe, dir string, headless bool) (*ownerChrome, error) {
		mu.Lock()
		s = append(s, dir)
		mu.Unlock()
		return &ownerChrome{port: port, headless: headless, done: make(chan struct{}), kill: func() {
			mu.Lock()
			k++
			mu.Unlock()
		}}, nil
	}
	t.Cleanup(func() { startOwnerChrome = old })
	return &s, &k
}

func TestOwnerChromeOnePerOwner(t *testing.T) {
	ambPerfilNet(t)
	starts, kills := llançadorPropietari(t)
	a1, err := launchChromeScoped(true, userScope("anna", "default"))
	if err != nil {
		t.Fatal(err)
	}
	a2, err := launchChromeScoped(true, userScope("anna", "viatge"))
	if err != nil {
		t.Fatal(err)
	}
	if len(*starts) != 1 {
		t.Fatalf("dues feines d'Anna han de compartir un sol Chrome: %v", *starts)
	}
	if a1.oc != a2.oc || a1 == a2 {
		t.Fatal("cada feina té el seu Browser (pestanya) sobre el Chrome comú")
	}
	b1, err := launchChromeScoped(true, userScope("bernat", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if len(*starts) != 2 || (*starts)[0] == (*starts)[1] || b1.oc == a1.oc {
		t.Fatalf("Bernat ha de tenir el seu Chrome i el seu perfil: %v", *starts)
	}
	if p := ProbeOrdinador(context.Background(), identity.Key("anna"), ""); p.ControlOwner != "agent" || p.Health != "running" || p.BrowserMode != "launched" {
		t.Fatalf("Anna amb el navegador obert: %+v", p)
	}
	a1.closeIntern()
	if *kills != 0 {
		t.Fatal("tancar una feina no pot tancar el navegador de l'altra")
	}
	a2.closeIntern()
	if *kills != 1 {
		t.Fatalf("l'última feina tanca el Chrome d'Anna: %d", *kills)
	}
	if p := ProbeOrdinador(context.Background(), identity.Key("anna"), ""); p.ControlOwner != "none" || p.BrowserMode != "on_demand" {
		t.Fatalf("Anna sense navegador: %+v", p)
	}
	b1.closeIntern()
}

// Chrome supervisat (VM): pestanya pròpia per feina i només el propietari.
func TestOrdinadorAttachOwnerGuard(t *testing.T) {
	ambPerfilNet(t)
	f := newFakeCDP(t)
	t.Cleanup(f.srv.Close)
	addr := strings.TrimPrefix(f.srv.URL, "http://")
	t.Setenv("GREGAL_BROWSER_CDP", addr)
	if _, err := openBrowser(BrowserModeHeadless, userScope("anna", "default")); err == nil {
		t.Fatal("sense propietari declarat, cap compte comparteix el Chrome supervisat")
	}
	if b, err := openBrowser(BrowserModeHeadless, "default"); err != nil {
		t.Fatalf("el principal local sí: %v", err)
	} else {
		b.closeIntern()
	}
	t.Setenv("GREGAL_ORDINADOR_OWNER", "anna")
	if _, err := openBrowser(BrowserModeHeadless, userScope("bernat", "default")); err == nil {
		t.Fatal("un altre usuari no pot fer servir l'ordinador d'Anna")
	}
	b, err := openBrowser(BrowserModeHeadless, userScope("anna", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if !b.attachTab {
		t.Fatal("al Chrome supervisat la feina obre una pestanya pròpia")
	}
	if p := ProbeOrdinador(context.Background(), identity.Key("anna"), ""); p.ControlOwner != "agent" || p.Health != "running" || p.BrowserMode != "attach" {
		t.Fatalf("salut amb pestanya: %+v", p)
	}
	b.closeIntern()
	if p := ProbeOrdinador(context.Background(), identity.Key("anna"), ""); p.ControlOwner != "none" {
		t.Fatalf("en tancar, ningú té el control: %+v", p)
	}
}

func TestProbeOrdinadorHealth(t *testing.T) {
	ambPerfilNet(t)
	f := newFakeCDP(t)
	t.Cleanup(f.srv.Close)
	addr := strings.TrimPrefix(f.srv.URL, "http://")
	files := t.TempDir()
	t.Setenv("GREGAL_BROWSER_CDP", addr)
	p := ProbeOrdinador(context.Background(), OrdinadorLocal, files)
	if p.Health != "running" || !hasAll(p.Capabilities, "browser", "files") {
		t.Fatalf("sa: %+v", p)
	}

	// Perfil del Chrome supervisat que no es pot escriure (un fitxer, no
	// una carpeta): la salut no pot dir running.
	notDir := filepath.Join(t.TempDir(), "fitxer")
	os.WriteFile(notDir, []byte("x"), 0o600)
	t.Setenv("GREGAL_ORDINADOR_PROFILE", notDir)
	if p = ProbeOrdinador(context.Background(), OrdinadorLocal, files); p.Health != "unreachable" || hasAll(p.Capabilities, "browser") {
		t.Fatalf("perfil no escrivible: %+v", p)
	}
	t.Setenv("GREGAL_ORDINADOR_PROFILE", t.TempDir())
	if p = ProbeOrdinador(context.Background(), OrdinadorLocal, files); p.Health != "running" {
		t.Fatalf("perfil del Chrome supervisat: %+v", p)
	}

	// Navegador aturat: res escolta al port.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := l.Addr().String()
	l.Close()
	t.Setenv("GREGAL_BROWSER_CDP", dead)
	oldStart := procStart
	procStart = procStart.Add(-ordinadorStartGrace * 2)
	t.Cleanup(func() { procStart = oldStart })
	p = ProbeOrdinador(context.Background(), OrdinadorLocal, files)
	if p.Health != "stopped" || hasAll(p.Capabilities, "browser") {
		t.Fatalf("aturat: %+v", p)
	}

	// CDP fora de loopback: mai es fa servir.
	t.Setenv("GREGAL_BROWSER_CDP", "192.0.2.1:9222")
	if p = ProbeOrdinador(context.Background(), OrdinadorLocal, files); p.Health != "unreachable" {
		t.Fatalf("CDP no loopback: %+v", p)
	}

	// Visor: noVNC present o no, i només a loopback.
	t.Setenv("GREGAL_BROWSER_CDP", addr)
	vl, _ := net.Listen("tcp", "127.0.0.1:0")
	defer vl.Close()
	t.Setenv("GREGAL_ORDINADOR_VNC", vl.Addr().String())
	if p = ProbeOrdinador(context.Background(), OrdinadorLocal, files); !hasAll(p.Capabilities, "viewer") {
		t.Fatalf("noVNC present: %+v", p)
	}
	t.Setenv("GREGAL_ORDINADOR_VNC", "0.0.0.0:6080")
	if p = ProbeOrdinador(context.Background(), OrdinadorLocal, files); hasAll(p.Capabilities, "viewer") {
		t.Fatalf("noVNC exposat no compta: %+v", p)
	}
}

func hasAll(list []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, l := range list {
			if l == w {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
