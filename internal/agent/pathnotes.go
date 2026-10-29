package agent

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Notes per camí: context que arriba TARD i sense cap volta de model.
//
// La idea és la d'OpenHands amb les seves regles per ruta: quan el model toca
// un fitxer, el RESULTAT de l'eina porta a sobre el que ja sabem d'aquell
// fitxer (una regla del projecte, una decisió, un parany). No hi ha cap crida
// al model ni cap subagent: el context apareix just quan fa falta.
//
// Per què això i no un investigador previ: en un servidor d'una sola ranura
// (llama.cpp local) el scout competint amb el coordinador empitjora el torn
// (mesurat: 58 s → 2m18s) perquè es disputen el motlle. Una nota enganxada al
// resultat no demana ranura.

const (
	// notesMaxPerInjeccio topa quantes notes porta un sol resultat: el model
	// ja té el fitxer a sobre, no cal enterrar-lo.
	notesMaxPerInjeccio = 3
	// notesMaxChars topa la mida del bloc sencer.
	notesMaxChars = 700
	// notesFitxer és el fitxer de notes que es versiona amb el projecte.
	notesFitxer = ".gregal/notes.yaml"
	// notesFitxerLocal és el mateix però per a coses d'aquesta màquina
	// (ports, rutes, claus d'entorn): va fora del control de versions.
	notesFitxerLocal = ".gregal/notes.local.yaml"
)

// PathNote és una nota per a un camí. Match accepta un glob: amb barra es
// prova contra el camí relatiu («internal/web/*» cobreix tota la carpeta) i
// sense barra contra el nom del fitxer («*.kt»).
type PathNote struct {
	Match string `yaml:"match" json:"match"`
	Note  string `yaml:"note" json:"note"`
}

type fitxerNotes struct {
	Notes []PathNote `yaml:"notes"`
}

// cacheNotes evita rellegir els fitxers a cada eina: es revalida amb el mtime.
type cacheNotes struct {
	mu    sync.Mutex
	clau  string
	notes []PathNote
}

var notesCache cacheNotes

// vistes recorda quines notes ja s'han injectat a cada sessió, per no repetir
// el mateix text a cada lectura del mateix fitxer.
var vistes = struct {
	mu sync.Mutex
	m  map[string]bool
}{m: map[string]bool{}}

// NotesDelProjecte torna les notes declarades al workspace. Un fitxer que no
// hi és o que no es pot llegir no és cap error: simplement no hi ha notes.
func NotesDelProjecte(workspace string) []PathNote {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil
	}
	var clau strings.Builder
	var notes []PathNote
	for _, rel := range []string{notesFitxer, notesFitxerLocal} {
		p := filepath.Join(ws, rel)
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		clau.WriteString(p)
		clau.WriteString(fi.ModTime().String())
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f fitxerNotes
		if yaml.Unmarshal(raw, &f) != nil {
			continue
		}
		notes = append(notes, f.Notes...)
	}
	if len(notes) == 0 {
		return nil
	}
	k := clau.String()
	notesCache.mu.Lock()
	defer notesCache.mu.Unlock()
	if notesCache.clau == k {
		return notesCache.notes
	}
	notesCache.clau, notesCache.notes = k, notes
	return notes
}

// rutaRelativa normalitza el camí que ha demanat el model a relatiu al
// workspace, amb barres. Si és fora del workspace, es queda com a base name.
func rutaRelativa(workspace, demanat string) string {
	p := strings.TrimSpace(demanat)
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) && strings.TrimSpace(workspace) != "" {
		p = filepath.Join(workspace, p)
	}
	if strings.TrimSpace(workspace) != "" {
		if rel, err := filepath.Rel(workspace, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

// encaixa diu si una nota és per a aquest camí. Els patrons amb barra es
// proven contra el camí sencer (i com a prefix de carpeta); els que no en
// tenen, contra el nom del fitxer.
func encaixa(patro, rel string) bool {
	patro = strings.TrimSpace(filepath.ToSlash(patro))
	if patro == "" || rel == "" {
		return false
	}
	if strings.Contains(patro, "/") {
		if ok, _ := path.Match(patro, rel); ok {
			return true
		}
		// «internal/web/*» també ha de cobrir «internal/web/x/y.go».
		if prefix := strings.TrimSuffix(patro, "*"); prefix != patro && strings.HasPrefix(rel, prefix) {
			return true
		}
		return patro == rel
	}
	if ok, _ := path.Match(patro, path.Base(rel)); ok {
		return true
	}
	ok, _ := path.Match(patro, rel)
	return ok
}

// filesPerEina diu quin camí toca una eina, o "" si no en toca cap (grep,
// glob, bash…). Només les eines que treballen sobre un fitxer concret.
func filesPerEina(name, argsJSON string) string {
	switch name {
	case "read", "edit", "patch", "write", "read_image", "office_read", "pdf_read":
	default:
		return ""
	}
	var a struct {
		Path string `json:"path"`
		File string `json:"file"`
	}
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return ""
	}
	if strings.TrimSpace(a.Path) != "" {
		return a.Path
	}
	return a.File
}

// NotaNotes construeix el bloc de notes per al resultat d'una eina: "" si no
// n'hi ha cap que encaixi o si ja s'havien injectat en aquesta sessió.
func NotaNotes(session, workspace, name, argsJSON string) string {
	demanat := filesPerEina(name, argsJSON)
	if demanat == "" {
		return ""
	}
	totes := NotesDelProjecte(workspace)
	if len(totes) == 0 {
		return ""
	}
	rel := rutaRelativa(workspace, demanat)
	var triades []string
	for _, n := range totes {
		if !encaixa(n.Match, rel) {
			continue
		}
		text := strings.TrimSpace(n.Note)
		if text == "" {
			continue
		}
		clau := session + "\x00" + rel + "\x00" + text
		vistes.mu.Lock()
		ja := vistes.m[clau]
		if !ja {
			vistes.m[clau] = true
		}
		vistes.mu.Unlock()
		if ja {
			continue
		}
		triades = append(triades, "- "+text)
		if len(triades) >= notesMaxPerInjeccio {
			break
		}
	}
	if len(triades) == 0 {
		return ""
	}
	bloc := "[notes] " + rel + "\n" + strings.Join(triades, "\n")
	if len(bloc) > notesMaxChars {
		bloc = bloc[:notesMaxChars] + "…"
	}
	log.Printf("notes: %d injectades a %s", len(triades), rel)
	return bloc
}

// OblidaNotes buida la memòria de notes injectades d'una sessió (en tancar-la).
func OblidaNotes(session string) {
	vistes.mu.Lock()
	defer vistes.mu.Unlock()
	prefix := session + "\x00"
	for k := range vistes.m {
		if strings.HasPrefix(k, prefix) {
			delete(vistes.m, k)
		}
	}
}

// PatronsDeNotes torna els patrons declarats, ordenats (per a docs i proves).
func PatronsDeNotes(notes []PathNote) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, filepath.ToSlash(strings.TrimSpace(n.Match)))
	}
	sort.Strings(out)
	return out
}
