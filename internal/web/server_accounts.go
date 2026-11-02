package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/identity"
	"gregal/internal/session"
)

// minPasswordLen és el mínim per a qualsevol compte (web, invitació i app).
// Vuit, com les contrasenyes que ja es feien servir: qui tria una de més llarga
// no rep cap avís. Un sol lloc mana, perquè el text de la web i la validació
// del servidor no es poden separar.
const minPasswordLen = 8

// Managed accounts are opt-in and stored separately from config.yaml so
// editing users never rewrites provider settings or exposes password hashes.
type managedAccount struct {
	Name     string   `json:"name"`
	Password string   `json:"password"`
	Revoked  []string `json:"revoked,omitempty"`
	// Admin: un compte gestionat també pot administrar el servidor (dues
	// persones que el porten a mitges). L'administrador del config ho és sempre.
	Admin bool `json:"admin,omitempty"`
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var managedName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{1,31}$`)

func managedRoot() string { return strings.TrimSpace(os.Getenv("GREGAL_ACCOUNTS_DIR")) }
func managedAccountsPath() string {
	return filepath.Join(filepath.Dir(session.DefaultDir()), "managed-accounts.json")
}
func readManaged() ([]managedAccount, error) {
	b, err := os.ReadFile(managedAccountsPath())
	if os.IsNotExist(err) {
		return []managedAccount{}, nil
	}
	if err != nil {
		return nil, err
	}
	var list []managedAccount
	if err = json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, a := range list {
		if !managedName.MatchString(a.Name) || a.Password == "" || seen[a.Name] {
			return nil, errors.New("invalid accounts file")
		}
		seen[a.Name] = true
	}
	return list, nil
}
func managedDefinition(root string, a managedAccount) config.UserCfg {
	dir := filepath.Join(root, identity.Key(a.Name))
	return config.UserCfg{Password: a.Password, Roots: []string{dir}, Home: dir, Admin: a.Admin}
}
func (u *Users) loadManagedAccounts() {
	root := managedRoot()
	if !filepath.IsAbs(root) {
		return
	}
	list, err := readManaged()
	if err != nil {
		return
	}
	for _, a := range list {
		for _, digest := range a.Revoked {
			u.revokedManaged[digest] = true
		}
		if _, exists := u.defs[a.Name]; !exists {
			u.defs[a.Name] = managedDefinition(root, a)
			u.managedNames[a.Name] = true
		}
	}
}
func writeManaged(list []managedAccount) error {
	path := managedAccountsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".accounts-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (s *Server) handleServerAccounts(w http.ResponseWriter, r *http.Request) {
	root := managedRoot()
	if !filepath.IsAbs(root) || s.users == nil {
		http.Error(w, "account management not configured", http.StatusNotFound)
		return
	}
	u := s.users
	// Refuse to introduce accounts into a server whose handler was started
	// without any authentication; that handler cannot require login until restart.
	if s.Token == "" && !u.Actiu() {
		http.Error(w, "authentication required before managing accounts", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		u.mu.RLock()
		defer u.mu.RUnlock()
		list, err := readManaged()
		if err != nil {
			http.Error(w, "cannot read accounts", 500)
			return
		}
		type entry struct {
			Name    string `json:"name"`
			Managed bool   `json:"managed"`
			Admin   bool   `json:"admin"`
		}
		managed := map[string]bool{}
		for _, a := range list {
			managed[a.Name] = u.managedNames[a.Name]
		}
		out := []entry{}
		for name, d := range u.defs {
			out = append(out, entry{Name: name, Managed: managed[name], Admin: d.Admin})
		}
		sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
		writeJSON(w, map[string]any{"accounts": out})
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Password string `json:"password"`
			Action   string `json:"action"`
			Admin    bool   `json:"admin"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if !managedName.MatchString(req.Name) || strings.EqualFold(req.Name, "local") {
			http.Error(w, "invalid username", 400)
			return
		}
		switch req.Action {
		case "", "create", "reset":
			if len(req.Password) < minPasswordLen || len(req.Password) > 1024 {
				http.Error(w, "invalid username or password (minimum "+strconv.Itoa(minPasswordLen)+" characters)", 400)
				return
			}
		case "delete", "admin":
		default:
			http.Error(w, "unknown action", 400)
			return
		}
		switch req.Action {
		case "reset":
			if code, err := u.resetManaged(req.Name, HashPassword(req.Password)); err != nil {
				http.Error(w, err.Error(), code)
				return
			}
			writeJSON(w, map[string]any{"name": req.Name})
		case "admin":
			if code, err := u.setManagedAdmin(req.Name, req.Admin); err != nil {
				http.Error(w, err.Error(), code)
				return
			}
			writeJSON(w, map[string]any{"name": req.Name, "admin": req.Admin})
		case "delete":
			if code, err := u.deleteManaged(req.Name); err != nil {
				http.Error(w, err.Error(), code)
				return
			}
			// The data stays on disk for the administrator, but a new account
			// with the same name must not inherit it.
			archived := archiveDir(managedDefinition(root, managedAccount{Name: req.Name}).Home) &&
				archiveDir(userStateDir(s.cfgPath, req.Name))
			writeJSON(w, map[string]any{"name": req.Name, "deleted": true, "archived": archived})
		default:
			hash := HashPassword(req.Password)
			u.mu.Lock()
			list, err := readManaged()
			code := 500
			if err == nil {
				code, err = u.createManagedLocked(root, req.Name, hash, list)
			} else {
				err = errors.New("cannot read accounts")
			}
			u.mu.Unlock()
			if err != nil {
				http.Error(w, err.Error(), code)
				return
			}
			writeJSON(w, map[string]any{"name": req.Name})
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), 405)
	}
}

// nameTakenLocked diu si el nom ja és d'algú, sense distingir majúscules:
// «Anna» i «anna» serien dues persones amb noms que ningú no sap separar.
// Caller holds u.mu.
func (u *Users) nameTakenLocked(name string, list []managedAccount) bool {
	for existing := range u.defs {
		if strings.EqualFold(existing, name) {
			return true
		}
	}
	for _, a := range list {
		if strings.EqualFold(a.Name, name) {
			return true
		}
	}
	return false
}

// dropTokensLocked tanca totes les sessions d'un usuari. Caller holds u.mu.
func (u *Users) dropTokensLocked(name string) (digests []string) {
	for token, owner := range u.tokens {
		if owner == name {
			digests = append(digests, tokenDigest(token))
			delete(u.tokens, token)
		}
	}
	return digests
}

// resetManaged canvia la contrasenya i tanca les sessions obertes. Les
// empremtes de les sessions tancades es desen amb la contrasenya nova només
// fins que el fitxer de tokens ja no les té: protegeixen d'una caiguda entre
// les dues escriptures, i després s'esborren perquè la llista no creixi.
func (u *Users) resetManaged(name, hash string) (int, error) {
	u.mu.Lock()
	if !u.managedNames[name] {
		u.mu.Unlock()
		return 404, errors.New("account is not managed here")
	}
	list, err := readManaged()
	if err != nil {
		u.mu.Unlock()
		return 500, errors.New("cannot read accounts")
	}
	i := managedIndex(list, name)
	if i < 0 {
		u.mu.Unlock()
		return 404, errors.New("account is not managed here")
	}
	var digests []string
	for token, owner := range u.tokens {
		if owner == name {
			digests = append(digests, tokenDigest(token))
		}
	}
	list[i].Password = hash
	list[i].Revoked = append(list[i].Revoked, digests...)
	if err := writeManaged(list); err != nil {
		u.mu.Unlock()
		return 500, errors.New("cannot save account")
	}
	for _, d := range u.dropTokensLocked(name) {
		u.revokedManaged[d] = true
	}
	d := u.defs[name]
	d.Password = hash
	u.defs[name] = d
	u.mu.Unlock()
	if u.saveTokens() == nil {
		u.forgetRevoked(name)
	}
	return 0, nil
}

// forgetRevoked esborra les empremtes d'un compte un cop el fitxer de tokens
// ja no conté cap d'aquelles sessions.
func (u *Users) forgetRevoked(name string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	list, err := readManaged()
	if err != nil {
		return
	}
	i := managedIndex(list, name)
	if i < 0 || len(list[i].Revoked) == 0 {
		return
	}
	for _, d := range list[i].Revoked {
		delete(u.revokedManaged, d)
	}
	list[i].Revoked = nil
	_ = writeManaged(list)
}

// deleteManaged treu un compte gestionat i en tanca les sessions. Un token
// d'un nom que ja no existeix no es torna a carregar, així que no cal
// recordar-ne cap empremta.
func (u *Users) deleteManaged(name string) (int, error) {
	u.mu.Lock()
	if !u.managedNames[name] {
		u.mu.Unlock()
		return 404, errors.New("account is not managed here")
	}
	list, err := readManaged()
	if err != nil {
		u.mu.Unlock()
		return 500, errors.New("cannot read accounts")
	}
	i := managedIndex(list, name)
	if i < 0 {
		u.mu.Unlock()
		return 404, errors.New("account is not managed here")
	}
	for _, d := range list[i].Revoked {
		delete(u.revokedManaged, d)
	}
	if err := writeManaged(append(list[:i:i], list[i+1:]...)); err != nil {
		u.mu.Unlock()
		return 500, errors.New("cannot save accounts")
	}
	delete(u.defs, name)
	delete(u.managedNames, name)
	u.dropTokensLocked(name)
	u.mu.Unlock()
	_ = u.saveTokens()
	return 0, nil
}

// setManagedAdmin fa (o desfà) administrador un compte gestionat. Els
// permisos es miren a cada petició: no cal tornar a entrar.
func (u *Users) setManagedAdmin(name string, admin bool) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.managedNames[name] {
		return 404, errors.New("account is not managed here")
	}
	list, err := readManaged()
	if err != nil {
		return 500, errors.New("cannot read accounts")
	}
	i := managedIndex(list, name)
	if i < 0 {
		return 404, errors.New("account is not managed here")
	}
	list[i].Admin = admin
	if err := writeManaged(list); err != nil {
		return 500, errors.New("cannot save account")
	}
	d := u.defs[name]
	d.Admin = admin
	u.defs[name] = d
	return 0, nil
}

// IsAdmin diu si el compte és administrador ara mateix (ok=false si no hi és).
func (u *Users) IsAdmin(name string) (admin, ok bool) {
	if u == nil {
		return false, false
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	d, ok := u.defs[name]
	return d.Admin, ok
}

func managedIndex(list []managedAccount, name string) int {
	for i := range list {
		if list[i].Name == name {
			return i
		}
	}
	return -1
}

// archiveDir aparta una carpeta d'un compte esborrat amb un nom que cap
// compte nou no pot reclamar. Torna true si no hi havia res o s'ha apartat.
func archiveDir(dir string) bool {
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return true
	}
	return os.Rename(dir, dir+".deleted-"+strconv.FormatInt(time.Now().Unix(), 10)) == nil
}

// Caller holds u.mu. The account registry is committed before it becomes live.
func (u *Users) createManagedLocked(root, name, hash string, list []managedAccount) (int, error) {
	if u.nameTakenLocked(name, list) {
		return 409, errors.New("account already exists")
	}
	account := managedAccount{Name: name, Password: hash}
	def := managedDefinition(root, account)
	if err := os.MkdirAll(def.Home, 0700); err != nil {
		return 500, errors.New("cannot create workspace")
	}
	info, err := os.Lstat(def.Home)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 500, errors.New("invalid workspace")
	}
	if err = writeManaged(append(list, account)); err != nil {
		return 500, errors.New("cannot save account")
	}
	u.defs[name] = def
	u.managedNames[name] = true
	return 0, nil
}
