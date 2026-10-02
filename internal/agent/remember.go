package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
)

// Sig resumeix una crida en una signatura estable per recordar aprovacions
// dins la sessió: eina + objectiu (ruta, comanda, URL...), no els detalls
// volàtils. Dues crides amb el mateix objectiu comparteixen signatura.
func Sig(name, argsJSON string) string {
	target := ""
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err == nil {
		str := func(k string) string {
			if v, ok := m[k].(string); ok {
				return strings.TrimSpace(v)
			}
			return ""
		}
		switch name {
		case "write", "edit", "read":
			// Per directori, no per fitxer: "sempre" en una carpeta de codi
			// cobreix tot el paquet sense obrir la resta del disc.
			target = dirOf(str("path"))
		case "bash":
			target = str("command")
		case "grep":
			target = str("pattern") + "\x00" + str("dir")
		case "glob":
			target = str("pattern") + "\x00" + str("dir")
		case "web_fetch":
			target = str("url")
		case "web_search":
			target = str("query")
		default:
			if isMCP(name) {
				// L'eina sencera: els args MCP varien massa per ser útils.
				target = name
			}
		}
	}
	if target == "" {
		target = strings.TrimSpace(argsJSON)
	}
	return name + "\x00" + target
}

// Remember guarda auto-permisos de sessió (només memòria, mai disc).
// Cada frontend fa servir el seu àmbit ("web", "tui", "tg:<chat>") i el
// neteja en canviar de sessió.
type Remember struct {
	mu     sync.Mutex
	scopes map[string]map[string]bool
}

func NewRemember() *Remember { return &Remember{scopes: map[string]map[string]bool{}} }

// dirOf normalitza el directori d'una ruta per a signatures ("", "." i
// buits col·lapsen a ".").
func dirOf(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "."
	}
	return filepath.Clean(filepath.Dir(p))
}

func (r *Remember) Allow(scope, sig string) {
	if r == nil || scope == "" || sig == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.scopes[scope]
	if !ok {
		m = map[string]bool{}
		r.scopes[scope] = m
	}
	m[sig] = true
}

func (r *Remember) Allowed(scope, sig string) bool {
	if r == nil || scope == "" || sig == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scopes[scope][sig]
}

func (r *Remember) Clear(scope string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.scopes, scope)
}

// DoomSig és la signatura per al detector de bucles (DoomTracker), no per
// als permisos. Sig agrupa write/edit/read per DIRECTORI (perquè un
// «sempre» cobreixi el paquet), i això feia que quatre write de fitxers
// diferents a la mateixa carpeta —una tasca normal— comptessin com la
// mateixa crida repetida i el tercer quedés bloquejat com a «eina
// repetida». Aquí es distingeix per fitxer i pel que s'hi fa: dues
// escriptures iguals del mateix contingut sí que són repetició; dues de
// fitxers diferents, no.
func DoomSig(name, argsJSON string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return Sig(name, argsJSON)
	}
	str := func(k string) string {
		if v, ok := m[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	switch name {
	case "write":
		return name + "\x00" + str("path") + "\x00" + capRunes(str("content"), 400)
	case "edit":
		return name + "\x00" + str("path") + "\x00" + capRunes(str("old_string"), 200) + "\x00" + capRunes(str("new_string"), 200)
	case "patch":
		raw, _ := json.Marshal(m["edits"])
		return name + "\x00" + str("path") + "\x00" + capRunes(string(raw), 400)
	case "read":
		return name + "\x00" + str("path") + "\x00" + strings.TrimSpace(argsJSON)
	}
	return Sig(name, argsJSON)
}
