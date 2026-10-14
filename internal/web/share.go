package web

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/llm"
	"gregal/internal/session"
	"gregal/internal/transcript"
)

// Enllaços de només lectura d'una conversa.
//
// POST /api/shares crea una instantània: l'HTML ja renderitzat i SEMPRE
// sanitzat (claus, secrets del config, camins personals i, si no es diu el
// contrari, correus). Es desa a <data>/shares/<id>.html amb les metadades
// a <id>.json. L'id són 20 bytes de crypto/rand (160 bits) en hex.
//
// GET /share/<id> la serveix sense token, igual que / i /app/: el model de
// seguretat del servidor protegeix /api/*, i l'enllaç és una capacitat (qui
// coneix l'id la pot llegir). Continua valent el guard d'escolta: sense
// token ni usuaris el servidor només escolta a loopback, i amb token o
// usuaris la pàgina és visible per qui arriba al servidor i té l'enllaç.
// Es serveix amb una CSP estricta (cap script) i nosniff.
//
// GET /api/shares llista les de l'usuari i DELETE /api/shares/{id} la
// revoca (esborra els fitxers: l'enllaç passa a 404).

// shareMeta és el que es desa al costat de cada instantània.
type shareMeta struct {
	ID      string    `json:"id"`
	Owner   string    `json:"owner,omitempty"`
	Session string    `json:"session,omitempty"`
	Title   string    `json:"title"`
	Created time.Time `json:"created"`
	Bytes   int       `json:"bytes"`
	Emails  bool      `json:"emails_hidden"`
}

var (
	reShareID = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sharesMu  sync.Mutex
)

// sharesDir és <data>/shares, al costat de sessions (respecta GREGAL_DATA_DIR).
func sharesDir() string { return filepath.Join(filepath.Dir(session.DefaultDir()), "shares") }

func newShareID() (string, error) {
	b := make([]byte, 20)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func readShareMeta(id string) (shareMeta, error) {
	var m shareMeta
	if !reShareID.MatchString(id) {
		return m, os.ErrNotExist
	}
	raw, err := os.ReadFile(filepath.Join(sharesDir(), id+".json"))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(raw, &m)
	return m, err
}

func listShares(owner string) []shareMeta {
	files, _ := filepath.Glob(filepath.Join(sharesDir(), "*.json"))
	out := []shareMeta{}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		m, err := readShareMeta(id)
		if err != nil || m.Owner != owner {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// handleShares: GET llista, POST crea.
func (s *Server) handleShares(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows := listShares(s.usuari())
		type row struct {
			shareMeta
			URL string `json:"url"`
		}
		out := make([]row, 0, len(rows))
		for _, m := range rows {
			out = append(out, row{m, "/share/" + m.ID})
		}
		writeJSON(w, map[string]any{"shares": out})
	case http.MethodPost:
		s.createShare(w, r)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Name és una conversa desada; buit = la conversa viva de la pestanya.
		Name string `json:"name"`
		// Emails amaga els correus (per defecte, sí).
		Emails *bool `json:"emails"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	var (
		meta  = transcript.Meta{}
		convo = []llm.Message(nil)
		name  = strings.TrimSpace(req.Name)
	)
	if name != "" {
		m, c, err := s.savedExport(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		meta, convo = m, c
	} else {
		meta, convo = s.liveExport()
		s.mu.Lock()
		name = s.convoFile
		s.mu.Unlock()
	}
	if len(convo) == 0 {
		http.Error(w, "empty conversation", http.StatusBadRequest)
		return
	}
	emails := req.Emails == nil || *req.Emails
	body, _, _, _ := s.renderExport("html", meta, convo, true, emails)
	id, err := newShareID()
	if err != nil {
		http.Error(w, "random source unavailable", http.StatusInternalServerError)
		return
	}
	sm := shareMeta{
		ID: id, Owner: s.usuari(), Session: name, Created: time.Now().UTC(), Bytes: len(body), Emails: emails,
		// El títol també va sanitzat: la llista el mostra.
		Title: transcript.Redact(meta.Title, s.redactOptions(emails)),
	}
	if err := writeShare(sm, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"id": id, "url": "/share/" + id, "title": sm.Title, "created": sm.Created})
}

func writeShare(m shareMeta, body string) error {
	sharesMu.Lock()
	defer sharesMu.Unlock()
	dir := sharesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, m.ID+".html"), []byte(body), 0o600); err != nil {
		return err
	}
	// Les metadades al final: sense .json l'enllaç no existeix.
	return os.WriteFile(filepath.Join(dir, m.ID+".json"), raw, 0o600)
}

// handleShareDelete revoca un enllaç propi.
func (s *Server) handleShareDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "DELETE only", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	m, err := readShareMeta(id)
	if err != nil || m.Owner != s.usuari() {
		http.NotFound(w, r)
		return
	}
	sharesMu.Lock()
	// Primer el .json: un cop fora, l'enllaç ja és mort encara que l'HTML
	// no es pugui esborrar.
	err = os.Remove(filepath.Join(sharesDir(), id+".json"))
	_ = os.Remove(filepath.Join(sharesDir(), id+".html"))
	sharesMu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"deleted": id})
}

// handleSharePage serveix una instantània (sense token: vegeu el comentari
// de dalt). Res de scripts: la CSP ho prohibeix encara que s'hi colés.
func handleSharePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := readShareMeta(id); err != nil {
		http.NotFound(w, r)
		return
	}
	raw, err := os.ReadFile(filepath.Join(sharesDir(), id+".html"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", transcript.HTMLCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	_, _ = w.Write(raw)
}
