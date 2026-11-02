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
	"strconv"
	"strings"

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
	return config.UserCfg{Password: a.Password, Roots: []string{dir}, Home: dir}
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
		writeJSON(w, map[string]any{"accounts": out})
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Password string `json:"password"`
			Action   string `json:"action"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if !managedName.MatchString(req.Name) || strings.EqualFold(req.Name, "local") || len(req.Password) < minPasswordLen || len(req.Password) > 1024 {
			http.Error(w, "invalid username or password (minimum "+strconv.Itoa(minPasswordLen)+" characters)", 400)
			return
		}
		hash := HashPassword(req.Password)
		u.mu.Lock()
		defer u.mu.Unlock()
		list, err := readManaged()
		if err != nil {
			http.Error(w, "cannot read accounts", 500)
			return
		}
		if req.Action == "reset" {
			if !u.managedNames[req.Name] {
				http.Error(w, "account is not managed here", 404)
				return
			}
			for i := range list {
				if list[i].Name != req.Name {
					continue
				}
				for token, name := range u.tokens {
					if name == req.Name {
						list[i].Revoked = append(list[i].Revoked, tokenDigest(token))
					}
				}
				list[i].Password = hash
				if err := writeManaged(list); err != nil {
					http.Error(w, "cannot save account", 500)
					return
				}
				for token, name := range u.tokens {
					if name == req.Name {
						u.revokedManaged[tokenDigest(token)] = true
						delete(u.tokens, token)
					}
				}
				d := u.defs[req.Name]
				d.Password = hash
				u.defs[req.Name] = d
				writeJSON(w, map[string]any{"name": req.Name})
				return
			}
			http.Error(w, "account is not managed here", 404)
			return
		}
		if req.Action != "" && req.Action != "create" {
			http.Error(w, "unknown action", 400)
			return
		}
		if code, err := u.createManagedLocked(root, req.Name, hash, list); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		writeJSON(w, map[string]any{"name": req.Name})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), 405)
	}
}

// Caller holds u.mu. The account registry is committed before it becomes live.
func (u *Users) createManagedLocked(root, name, hash string, list []managedAccount) (int, error) {
	if _, exists := u.defs[name]; exists {
		return 409, errors.New("account already exists")
	}
	for _, a := range list {
		if a.Name == name {
			return 409, errors.New("account already exists")
		}
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
