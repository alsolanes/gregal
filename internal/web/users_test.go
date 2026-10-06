package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gregal/internal/config"
)

// usuarisProva fabrica un registre amb dos comptes i els tokens en un
// fitxer temporal: cap prova ha de tocar el tokens.json de debò.
func usuarisProva(t *testing.T) *Users {
	t.Helper()
	cfg := &config.Config{Users: map[string]config.UserCfg{
		"usera": {Password: HashPassword("clau-usera"), Roots: []string{"/tmp/usera"}},
		"userb": {Password: HashPassword("clau-userb"), Roots: []string{"/tmp/userb"}},
	}}
	u := NewUsers(cfg, "")
	u.path = filepath.Join(t.TempDir(), "tokens.json")
	return u
}

// TestPasswordHash: la contrasenya mai es desa en clar i la comprovació
// és exacta.
func TestPasswordHash(t *testing.T) {
	h := HashPassword("la-meva-clau")
	if strings.Contains(h, "la-meva-clau") {
		t.Fatal("la contrasenya ha sortit al hash")
	}
	if !strings.HasPrefix(h, "pbkdf2$sha256$") {
		t.Fatalf("format inesperat: %s", h)
	}
	if !VerifyPassword(h, "la-meva-clau") {
		t.Fatal("la clau bona hauria de validar")
	}
	if VerifyPassword(h, "la-meva-clau ") || VerifyPassword(h, "LA-MEVA-CLAU") {
		t.Fatal("una clau diferent no ha de validar")
	}
	// Dues vegades el mateix text → salts diferents → hashos diferents.
	if HashPassword("x") == HashPassword("x") {
		t.Fatal("els hashes haurien d'anar amb salt")
	}
}

// TestVerifyPasswordRebuig: res d'estrany no ha de validar mai (fail closed).
func TestVerifyPasswordRebuig(t *testing.T) {
	casos := []string{"", "text pla", "pbkdf2$sha256$", "pbkdf2$sha256$0$aa$bb",
		"pbkdf2$md5$10$aa$bb", "pbkdf2$sha256$99999999$aa$bb", "$$$$"}
	for _, c := range casos {
		if VerifyPassword(c, "") || VerifyPassword(c, "qualsevol") {
			t.Fatalf("no hauria de validar: %q", c)
		}
	}
}

// TestLoginEmetToken: entrar torna un token que identifica l'usuari, i la
// contrasenya dolenta no en torna cap.
func TestLoginEmetToken(t *testing.T) {
	u := usuarisProva(t)
	tok, usu, err := u.Login("userb", "clau-userb")
	if err != nil || usu.Name != "userb" {
		t.Fatalf("login: %v %v", usu, err)
	}
	if len(tok) < 20 {
		t.Fatalf("token massa curt: %q", tok)
	}
	got, ok := u.Resolve(tok)
	if !ok || got.Name != "userb" || len(got.Roots) != 1 {
		t.Fatalf("resolució: %+v %v", got, ok)
	}
	if _, _, err := u.Login("userb", "equivocada"); err == nil {
		t.Fatal("amb la clau dolenta no hauria d'entrar")
	}
	if _, _, err := u.Login("ningu", "clau-userb"); err == nil {
		t.Fatal("un usuari que no existeix no hauria d'entrar")
	}
	// Un token inventat no resol.
	if _, ok := u.Resolve("token-inventat"); ok {
		t.Fatal("un token inventat no hauria de resoldre")
	}
}

// TestLogoutOblidaToken: després de sortir, el token ja no val.
func TestLogoutOblidaToken(t *testing.T) {
	u := usuarisProva(t)
	tok, _, _ := u.Login("usera", "clau-usera")
	u.Logout(tok)
	if _, ok := u.Resolve(tok); ok {
		t.Fatal("el token sortit encara val")
	}
}

func TestSetPasswordTightensExistingFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	u := usuarisProva(t)
	u.pwPath = filepath.Join(t.TempDir(), "passwords.json")
	if err := os.WriteFile(u.pwPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(u.pwPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := u.SetPassword("usera", "new-password"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(u.pwPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("password file mode = %04o, want 0600", got)
	}
}

// TestLoginIntentsLimita: vuit intents fallits i l'adreça queda fora una
// estona.
func TestLoginIntentsLimita(t *testing.T) {
	u := usuarisProva(t)
	ip := "10.0.0.9"
	if !u.Permes(ip) {
		t.Fatal("hauria de començar permesa")
	}
	for i := 0; i < loginMaxIntents; i++ {
		u.Fallit(ip)
	}
	if u.Permes(ip) {
		t.Fatal("després de vuit intents no hauria de poder provar")
	}
	u.Encertat(ip)
	if !u.Permes(ip) {
		t.Fatal("qui encerta hauria de tornar a tenir via lliure")
	}
	// Una altra adreça no en queda afectada.
	if !u.Permes("10.0.0.10") {
		t.Fatal("el blocatge és per adreça")
	}
}

// TestSenseUsuarisTokenUnic: sense secció `users:` tot queda com abans:
// el token únic val com a admin sense límits.
func TestSenseUsuarisTokenUnic(t *testing.T) {
	cfg := &config.Config{}
	u := NewUsers(cfg, "")
	u.path = filepath.Join(t.TempDir(), "tokens.json")
	if u.Actiu() {
		t.Fatal("sense usuaris no hauria d'estar actiu")
	}
	u.SetLegacyToken("secret-de-sempre")
	got, ok := u.Resolve("secret-de-sempre")
	if !ok || !got.Admin || !got.Tothom {
		t.Fatalf("l'admin de sempre hauria de passar: %+v %v", got, ok)
	}
	if err := got.Allow("/home/usera/el/que/sigui"); err != nil {
		t.Fatalf("l'admin de sempre va a tot arreu: %v", err)
	}
	if _, ok := u.Resolve("altre"); ok {
		t.Fatal("un token diferent no hauria de passar")
	}
}

// TestLoginHTTP: la porta /api/login i qui és cada token.
func TestLoginHTTP(t *testing.T) {
	s := goalTestServer(t)
	s.users = usuarisProva(t)
	s.SetToken("")

	cos := strings.NewReader(`{"user":"userb","password":"clau-userb"}`)
	rec := httptest.NewRecorder()
	s.handleLogin(rec, httptest.NewRequest(http.MethodPost, "/api/login", cos))
	if rec.Code != 200 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"user":"userb"`) || !strings.Contains(body, `"token":"`) {
		t.Fatalf("resposta de login inesperada: %s", body)
	}

	// Amb la clau malament, 401 i cap token.
	rec = httptest.NewRecorder()
	s.handleLogin(rec, httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"user":"userb","password":"no"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("clau dolenta: %d", rec.Code)
	}

	// GET no val: la porta només accepta POST.
	rec = httptest.NewRecorder()
	s.handleLogin(rec, httptest.NewRequest(http.MethodGet, "/api/login", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET a /api/login: %d", rec.Code)
	}
}

// TestRequireUserAmbUsuaris: amb comptes, sense token no es passa; amb el
// token de l'usuari, sí, i /api/me diu qui ets.
func TestRequireUserAmbUsuaris(t *testing.T) {
	s := goalTestServer(t)
	s.users = usuarisProva(t)
	s.SetToken("")
	h := s.requireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r)
		if u == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(u.Name))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sense token hauria de ser 401: %d", rec.Code)
	}

	tok, _, _ := s.users.Login("userb", "clau-userb")
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "userb" {
		t.Fatalf("amb token de userb: %d %q", rec.Code, rec.Body.String())
	}

	// /api/login ha de quedar oberta: és la porta.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login", nil))
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("/api/login no pot demanar token")
	}
}
