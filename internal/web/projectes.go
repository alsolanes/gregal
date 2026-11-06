package web

import (
	"os"
	"path/filepath"
	"strings"
)

// On treballen els vents quan fan projectes.
//
// El repo del Gregal (~/gregal-live) és el codi del Gregal: una web, un joc o
// un script que demana la persona no hi van, perquè queden barrejats amb el
// codi del servei (el joc del 31/10 va acabar a l'arrel del repo). Els
// projectes tenen la seva carpeta: ~/gregal/projectes/<nom>.
// projectesDir és la carpeta dels projectes dels vents. Es pot canviar amb
// GREGAL_PROJECTES (tests, o algú que la vulgui en un altre disc). Es llegeix
// a cada crida: l'entorn pot canviar i una caché global entre tests menteix.
func projectesDir() string {
	dir := strings.TrimSpace(os.Getenv("GREGAL_PROJECTES"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, "gregal", "projectes")
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir
}

// asseguraProjectes crea la carpeta de projectes si no hi és i la retorna.
// Si no es pot crear (permisos, disc), torna buit: mai un error dur per això.
func asseguraProjectes() string {
	dir := projectesDir()
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	return dir
}

// esRepoGregal diu si una carpeta és el codi del Gregal (i per tant no és un
// lloc on deixar projectes). Es mira el go.mod: el mòdul és `gregal`.
func esRepoGregal(dir string) bool {
	if dir == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "module ")) == "gregal"
		}
	}
	return false
}

// esCarpetaMassaAmpla: la carpeta d'usuari, la que les conté o l'arrel d'un
// disc no són un projecte. Una colla hi feia una còpia privada sencera per a
// cada membre (desenes de GB, minuts sense resposta) i els fitxers nous
// quedaven escampats per l'usuari.
func esCarpetaMassaAmpla(dir string) bool {
	if dir == "" {
		return false
	}
	clean := filepath.Clean(dir)
	if filepath.Dir(clean) == clean { // arrel del disc
		return true
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		h := filepath.Clean(home)
		if strings.EqualFold(clean, h) || strings.EqualFold(clean, filepath.Dir(h)) {
			return true
		}
	}
	return false
}

// ventWorkspace és la carpeta on treballa un vent en aquesta sessió: la que
// hagi triat la persona i, si no n'ha triat cap (o sigui, la sessió apunta al
// repo del Gregal), la carpeta de projectes.
func (s *Server) ventWorkspace() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ventWorkspaceLocked()
}

// ventWorkspaceLocked és el mateix amb el pany ja pres.
func (s *Server) ventWorkspaceLocked() (ws, repo string) {
	cwd := s.cwd
	if cwd == "" || esRepoGregal(cwd) || esCarpetaMassaAmpla(cwd) {
		if p := asseguraProjectes(); p != "" {
			return p, cwd
		}
	}
	return cwd, cwd
}

// windProjectesBrief diu al vent, amb rutes de debò, on ha de deixar els
// projectes i quina carpeta té en aquest torn. Els noms simbòlics («<projectes>»)
// no serveixen: el model ha de saber la ruta exacta per fer-hi mkdir i escriure-hi.
func windProjectesBrief(ws string) string {
	dir := projectesDir()
	if dir == "" || ws == "" {
		return ""
	}
	out := "\n\nWORKING FOLDER FOR THIS TURN: " + ws
	if dir != ws {
		out += "\nPROJECTS FOLDER: " + dir + " (a new project is a folder inside it)"
	} else {
		out += "\nPROJECTS FOLDER: this is it — a new project is a folder inside it"
	}
	return out
}
