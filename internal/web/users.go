package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/session"
)

// ErrCredencials són unes credencials que no quadren. No es diu mai si el
// que falla és l'usuari o la contrasenya: això regala la llista d'usuaris.
var ErrCredencials = errors.New("usuari o contrasenya incorrectes")

// ErrSenseUsuaris és quan el config no té cap secció `users:`.
var ErrSenseUsuaris = errors.New("no hi ha usuaris configurats")

const (
	pbkdf2Iters  = 120_000
	pbkdf2KeyLen = 32
	// tokenBytes són els bytes aleatoris d'un token de sessió (base64url).
	tokenBytes = 24
)

// User és algú que ha entrat: qui és, on pot treballar i si mana.
type User struct {
	Name string
	// Roots són les carpetes on aquest usuari pot navegar, obrir, editar i
	// executar. A fora, l'API de fitxers diu que no.
	Roots []string
	// Admin veu els endpoints compartits del servidor (jobs, GitHub) i pot
	// entrar amb el token únic de tota la vida.
	Admin bool
	// Home és on comencen les sessions de codi (les carpetes de treball
	// de l'usuari). Buit = la primera arrel.
	Home string
	// Tothom és el cas «aquest servidor no té usuaris configurats»: el
	// token únic mana i va a tot arreu, exactament com abans d'aquesta
	// feina. Un usuari de debò amb roots buides NO navega.
	Tothom bool
}

// Users valida contrasenyes, emet tokens de sessió i els resol a usuaris.
//
// Conviu amb el token únic de sempre (`Token` al config): si existeix, val
// com a l'usuari admin i tot queda exactament com abans d'aquesta feina. Els
// usuaris són una capa de més a sobre, no un canvi de les regles del joc.
type Users struct {
	mu     sync.RWMutex
	defs   map[string]config.UserCfg
	tokens map[string]string // token → nom d'usuari
	path   string            // fitxer de tokens emesos (0600)
	pwPath string            // fitxer de contrasenyes (0600)

	// intents compten els intents fallits per adreça, perquè una
	// contrasenya no es pugui anar provant a dojo.
	intents map[string]*intentsIP

	legacyToken string
	legacyUser  *User
}

// intentsIP és el comptador d'una adreça: quants intents fallits i des de
// quan. La finestra és de 5 minuts i el límit, 8.
type intentsIP struct {
	n      int
	primer time.Time
}

const (
	loginMaxIntents = 8
	loginFinestra   = 5 * time.Minute
)

// Permes diu si aquesta adreça encara pot provar d'entrar.
func (u *Users) Permes(ip string) bool {
	if u == nil {
		return true
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	c := u.intents[ip]
	if c == nil {
		return true
	}
	if time.Since(c.primer) > loginFinestra {
		delete(u.intents, ip)
		return true
	}
	return c.n < loginMaxIntents
}

// Fallit apunta un intent fallit.
func (u *Users) Fallit(ip string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.intents == nil {
		u.intents = map[string]*intentsIP{}
	}
	c := u.intents[ip]
	if c == nil || time.Since(c.primer) > loginFinestra {
		u.intents[ip] = &intentsIP{n: 1, primer: time.Now()}
		return
	}
	c.n++
}

// Encertat esborra el comptador: qui encerta, comença de zero.
func (u *Users) Encertat(ip string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.intents, ip)
}

// NewUsers construeix el registre a partir del config. `cfgPath` només
// serveix per saber on desar els tokens emesos.
func NewUsers(cfg *config.Config, cfgPath string) *Users {
	u := &Users{
		defs:    map[string]config.UserCfg{},
		tokens:  map[string]string{},
		pwPath:  passwordsPath(),
		intents: map[string]*intentsIP{},
		path:    tokensPath(),
		legacyUser: &User{
			Name:  "local",
			Admin: true,
		},
	}
	if cfg != nil {
		for name, d := range cfg.Users {
			n := strings.TrimSpace(name)
			if n == "" {
				continue
			}
			u.defs[n] = d
		}
	}
	// Sense cap usuari configurat, el token únic és el món de sempre:
	// l'admin va a tot arreu. Amb usuaris, l'admin també queda limitat a
	// les seves arrels (si no en té, no navega).
	if len(u.defs) == 0 {
		u.legacyUser.Tothom = true
	}
	// Si el config defineix l'usuari admin, se li prenen les arrels i el
	// nom del config; si no, l'admin pot anar a tot arreu (com sempre).
	if d, ok := u.defs[u.legacyUser.Name]; ok {
		u.legacyUser.Roots = netejaArrels(d.Roots)
		u.legacyUser.Home = netejaUn(d.Home)
		if d.Admin {
			u.legacyUser.Admin = true
		}
	}
	u.loadPasswords()
	u.loadTokens()
	return u
}

// SetLegacyToken fa que el token únic de sempre (--token / GREGAL_API_TOKEN)
// valgui com l'usuari admin. El servidor el fixa en arrencar.
func (u *Users) SetLegacyToken(tok string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.legacyToken = strings.TrimSpace(tok)
	u.mu.Unlock()
}

// Actiu diu si hi ha cap manera d'entrar configurada.
func (u *Users) Actiu() bool { return u != nil && len(u.defs) > 0 }

// Noms torna els usuaris configurats, ordenats.
func (u *Users) Noms() []string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	out := make([]string, 0, len(u.defs))
	for n := range u.defs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Login comprova les credencials i, si quadren, torna un token de sessió.
func (u *Users) Login(name, pw string) (string, *User, error) {
	name = strings.TrimSpace(name)
	u.mu.RLock()
	d, ok := u.defs[name]
	u.mu.RUnlock()
	if !ok || len(u.defs) == 0 {
		// Contra l'enumeració d'usuaris: es paga el mateix temps de PBKDF2
		// encara que el nom no existeixi.
		VerifyPassword("", pw)
		return "", nil, ErrCredencials
	}
	if !VerifyPassword(d.Password, pw) {
		return "", nil, ErrCredencials
	}
	tok, err := tokenNou()
	if err != nil {
		return "", nil, err
	}
	u.mu.Lock()
	u.tokens[tok] = name
	u.mu.Unlock()
	u.saveTokens()
	return tok, u.Usuari(name), nil
}

// Logout oblida un token emès.
func (u *Users) Logout(tok string) {
	if u == nil || tok == "" {
		return
	}
	u.mu.Lock()
	_, hi := u.tokens[tok]
	delete(u.tokens, tok)
	u.mu.Unlock()
	if hi {
		u.saveTokens()
	}
}

// Resolve torna l'usuari d'un token. El token únic del config val com a
// admin; la resta han d'haver passat per Login.
func (u *Users) Resolve(tok string) (*User, bool) {
	if u == nil || tok == "" {
		return nil, false
	}
	u.mu.RLock()
	legacy := u.legacyToken
	u.mu.RUnlock()
	if legacy != "" && subtle.ConstantTimeCompare([]byte(legacy), []byte(tok)) == 1 {
		return u.legacyUser, true
	}
	u.mu.RLock()
	name, ok := u.tokens[tok]
	u.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return u.Usuari(name), true
}

// Usuari construeix la vista pública d'un compte.
func (u *Users) Usuari(name string) *User {
	u.mu.RLock()
	d, ok := u.defs[name]
	u.mu.RUnlock()
	if !ok {
		return &User{Name: name}
	}
	return &User{Name: name, Roots: netejaArrels(d.Roots), Home: netejaUn(d.Home), Admin: d.Admin}
}

// --- tokens al disc ---

func tokensPath() string {
	return filepath.Join(filepath.Dir(session.DefaultDir()), "tokens.json")
}

// passwordsPath és el fitxer de contrasenyes, a part del config. Així
// `gregal passwd` no ha de reescriure el YAML (que té comentaris que valen
// molt) i les claus queden en un fitxer propi a 0600.
func passwordsPath() string {
	return filepath.Join(filepath.Dir(session.DefaultDir()), "passwords.json")
}

// loadPasswords llegeix el fitxer de contrasenyes i les barreja amb les que
// vinguin escrites al config (aquestes manen si hi són).
func (u *Users) loadPasswords() {
	raw, err := os.ReadFile(passwordsPath())
	if err != nil {
		return
	}
	m := map[string]string{}
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	u.mu.Lock()
	for name, hash := range m {
		d, ok := u.defs[name]
		if !ok {
			continue
		}
		if strings.TrimSpace(d.Password) == "" {
			d.Password = hash
			u.defs[name] = d
		}
	}
	u.mu.Unlock()
}

// SetPassword desa la contrasenya d'un usuari (ja hashejada) al fitxer de
// contrasenyes. Torna error si l'usuari no existeix al config.
func (u *Users) SetPassword(name, pw string) error {
	name = strings.TrimSpace(name)
	u.mu.Lock()
	_, ok := u.defs[name]
	u.mu.Unlock()
	if !ok {
		return fmt.Errorf("l'usuari %q no és al config (secció users:)", name)
	}
	m := map[string]string{}
	if raw, err := os.ReadFile(u.pwPath); err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	m[name] = HashPassword(pw)
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(u.pwPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(u.pwPath, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	u.mu.Lock()
	d := u.defs[name]
	d.Password = m[name]
	u.defs[name] = d
	u.mu.Unlock()
	return nil
}

func (u *Users) loadTokens() {
	raw, err := os.ReadFile(u.path)
	if err != nil {
		return
	}
	m := map[string]string{}
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	u.mu.Lock()
	for tok, name := range m {
		if _, ok := u.defs[name]; ok {
			u.tokens[tok] = name
		}
	}
	u.mu.Unlock()
}

// saveTokens desa els tokens emesos perquè un reinici del servidor no
// expulsi tothom. El fitxer va a 0600: un token és una clau.
func (u *Users) saveTokens() {
	u.mu.RLock()
	cp := make(map[string]string, len(u.tokens))
	for k, v := range u.tokens {
		cp[k] = v
	}
	u.mu.RUnlock()
	raw, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(u.path), 0o700); err != nil {
		return
	}
	tmp := u.path + ".tmp"
	if os.WriteFile(tmp, append(raw, '\n'), 0o600) != nil {
		return
	}
	_ = os.Rename(tmp, u.path)
}

func tokenNou() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- contrasenyes ---

// HashPassword torna `pbkdf2$sha256$iters$salt$hash` en base64.
func HashPassword(pw string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return ""
	}
	key := pbkdf2SHA256([]byte(pw), salt, pbkdf2Iters, pbkdf2KeyLen)
	return fmt.Sprintf("pbkdf2$sha256$%d$%s$%s", pbkdf2Iters,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

// VerifyPassword compara una contrasenya amb el que hi ha al config. Un
// format desconegut o buit no valida mai (fail closed).
func VerifyPassword(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return false
	}
	iters, err := strconv.Atoi(parts[2])
	if err != nil || iters <= 0 || iters > 5_000_000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(salt) < 8 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(want) == 0 {
		return false
	}
	got := pbkdf2SHA256([]byte(pw), salt, iters, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// pbkdf2SHA256 és PBKDF2 (RFC 2898) amb HMAC-SHA256. La stdlib no en porta
// i aquí no volem dependències: són vint línies i es poden llegir.
func pbkdf2SHA256(pw, salt []byte, iters, keyLen int) []byte {
	hashLen := sha256.Size
	blocs := (keyLen + hashLen - 1) / hashLen
	out := make([]byte, 0, blocs*hashLen)
	var idx [4]byte
	for i := 1; i <= blocs; i++ {
		binary.BigEndian.PutUint32(idx[:], uint32(i))
		mac := hmac.New(sha256.New, pw)
		mac.Write(salt)
		mac.Write(idx[:])
		u := mac.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for n := 1; n < iters; n++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// netejaUn deixa una carpeta en absolut (o buida si no hi és).
func netejaUn(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	u := netejaArrels([]string{p})
	if len(u) == 0 {
		return ""
	}
	return u[0]
}

// netejaArrels passa les arrels a absolutes i en treu les repetides.
func netejaArrels(in []string) []string {
	out := make([]string, 0, len(in))
	vist := map[string]bool{}
	for _, r := range in {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.HasPrefix(r, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				r = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(r, "~"), "/"))
			}
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		if vist[abs] {
			continue
		}
		vist[abs] = true
		out = append(out, abs)
	}
	return out
}
