package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/session"
)

// Invitation codes are random, single-use and stored only as hashes. They are
// consumed before account creation so a crash cannot make a code reusable.
var invitationMu sync.Mutex

func invitationDir() string { return filepath.Join(filepath.Dir(session.DefaultDir()), "invitations") }
func (s *Server) handleServerInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	if !filepath.IsAbs(managedRoot()) || s.users == nil || (s.Token == "" && !s.users.Actiu()) {
		http.Error(w, "account management unavailable", 503)
		return
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		http.Error(w, "cannot create invitation", 500)
		return
	}
	code := hex.EncodeToString(raw[:])
	until := time.Now().UTC().Add(30 * time.Minute)
	invitationMu.Lock()
	defer invitationMu.Unlock()
	dir := invitationDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		http.Error(w, "cannot create invitation", 500)
		return
	}
	// Expired invitations are not reusable and can be removed on the next issue.
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || len(e.Name()) != 64 {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
			if err == nil && n < time.Now().Unix() {
				_ = os.Remove(path)
			}
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, tokenDigest(code)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		http.Error(w, "cannot create invitation", 500)
		return
	}
	_, err = f.WriteString(strconv.FormatInt(until.Unix(), 10))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(dir, tokenDigest(code)))
		http.Error(w, "cannot create invitation", 500)
		return
	}
	writeJSON(w, map[string]string{"code": code, "expires": until.Format(time.RFC3339)})
}
func (s *Server) handleServerRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	if !filepath.IsAbs(managedRoot()) || s.users == nil || (s.Token == "" && !s.users.Actiu()) {
		http.Error(w, "registration unavailable", 503)
		return
	}
	ip := clientIP(r)
	if !s.users.Permes(ip) {
		http.Error(w, "too many attempts", 429)
		return
	}
	var req struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if len(req.Code) != 32 || !windCardID.MatchString(req.Code) || !managedName.MatchString(req.Name) || strings.EqualFold(req.Name, "local") || len(req.Password) < 12 || len(req.Password) > 1024 {
		s.users.Fallit(ip)
		http.Error(w, "invalid invitation or account details", 400)
		return
	}
	hash := HashPassword(req.Password)
	invitationMu.Lock()
	defer invitationMu.Unlock()
	path := filepath.Join(invitationDir(), tokenDigest(req.Code))
	b, err := os.ReadFile(path)
	if err != nil {
		s.users.Fallit(ip)
		http.Error(w, "invalid or expired invitation", 401)
		return
	}
	expires, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || expires <= time.Now().Unix() {
		s.users.Fallit(ip)
		http.Error(w, "invalid or expired invitation", 401)
		return
	}
	u := s.users
	u.mu.Lock()
	defer u.mu.Unlock()
	list, err := readManaged()
	if err != nil {
		http.Error(w, "cannot read accounts", 500)
		return
	}
	if _, ok := u.defs[req.Name]; ok {
		http.Error(w, "account already exists", 409)
		return
	}
	for _, a := range list {
		if a.Name == req.Name {
			http.Error(w, "account already exists", 409)
			return
		}
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "cannot use invitation", 500)
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		http.Error(w, "invalid or expired invitation", 401)
		return
	}
	if status, err := u.createManagedLocked(managedRoot(), req.Name, hash, list); err != nil {
		http.Error(w, "could not create account; request a new invitation", status)
		return
	}
	writeJSON(w, map[string]string{"name": req.Name})
}
